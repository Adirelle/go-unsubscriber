package main

import (
	"bufio"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/juju/loggo"
	"mime"
	"net"
	"net/mail"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
)

const (
	UnsubscribedFlag = "unsubscribed"
)

type (
	UnsubscribeInfo struct {
		To      *mail.Address
		Subject string
		Link    *url.URL
	}

	MailReader struct {
		C <-chan UnsubscribeInfo

		client  *client.Client
		output  chan UnsubscribeInfo
		decoder mime.WordDecoder
		loggo.Logger
	}
)

var (
	fetchSectionName *imap.BodySectionName
	linkRegex        *regexp.Regexp
)

func init() {
	var err error
	fetchSectionName, err = imap.ParseBodySectionName("BODY.PEEK[HEADER.FIELDS (To Subject List-Unsubscribe)]")
	if err != nil {
		panic(err.Error())
	}
	linkRegex = regexp.MustCompile("<((?:https?:|mailto:)[^>]+?)>")
}

func NewMailReader(config IMAPConfig) (r *MailReader, err error) {
	r = new(MailReader)
	r.Logger = loggo.GetLogger("imap")
	r.output = make(chan UnsubscribeInfo, 5)
	r.C = r.output

	err = r.connect(config)
	if err == nil {
		err = r.start()
	}

	return
}

func (r *MailReader) Close() error {
	if r.client != nil {
		r.Debugf("closing connection")
		return r.client.Logout()
	}
	return nil
}

func (r *MailReader) connect(config IMAPConfig) error {
	if conn, err := config.Connect(
		func(conn net.Conn) (interface{}, error) {
			return client.New(&ConnTap{conn, r.Logger.Child("wire")})
		},
	); err == nil {
		r.client = conn.(*client.Client)
	} else {
		return fmt.Errorf("could not connect to IMAP server: %w", err)
	}

	if _, err := r.client.Select(config.Mailbox, false); err != nil {
		return fmt.Errorf("could not select mailbox %q: %w", config.Mailbox, err)
	}

	return nil
}

func (r *MailReader) start() error {
	uids, err := r.searchMessages()
	if err != nil {
		return fmt.Errorf("could read mailbox: %w", err)
	}

	if !uids.Empty() {
		go r.fetchMails(uids)
	} else {
		close(r.output)
	}

	return nil
}

func (r *MailReader) searchMessages() (*imap.SeqSet, error) {

	criteria := imap.NewSearchCriteria()
	criteria.SeqNum, _ = imap.ParseSeqSet("1:20")
	criteria.Header.Set("List-Unsubscribe", "")
	criteria.WithoutFlags = []string{imap.DeletedFlag}

	r.Debugf("searching messages with these criteria: %s", criteria.Format())
	uniqueIds, err := r.client.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("could not search for messages: %w", err)
	}
	r.Infof("found %d messages", len(uniqueIds))

	var seqSet imap.SeqSet
	seqSet.AddNum(uniqueIds...)

	return &seqSet, nil
}

func (r *MailReader) fetchMails(uids *imap.SeqSet) {
	r.Debugf("fetching messages")
	messages := make(chan *imap.Message, 10)

	go func() {
		defer close(r.output)
		for message := range messages {
			r.processMessage(message)
		}
	}()

	err := r.client.UidFetch(uids, []imap.FetchItem{fetchSectionName.FetchItem(), imap.FetchFlags}, messages)
	if err != nil {
		r.Warningf("could not fetch (all) mail(s): %s", err)
	}

	r.Infof("all messages processed")
}

func (r *MailReader) processMessage(message *imap.Message) {
	r.Debugf("processing message #%d", message.Uid)

	reader := textproto.NewReader(bufio.NewReader(message.GetBody(fetchSectionName)))
	headers, err := reader.ReadMIMEHeader()
	if err != nil {
		r.Infof("could not parse MIME headers: %w", err)
		return
	}

	allValues := strings.Join(headers["List-Unsubscribe"], ",")
	matches := linkRegex.FindAllStringSubmatch(allValues, -1)
	if matches == nil {
		r.Infof("no valid List-Unsubscribe links found")
		return
	}

	to := r.decodeHeader(headers, "To")
	subject := r.decodeHeader(headers, "Subject")

	toAddr, err := mail.ParseAddress(to)
	if err != nil {
		r.Infof("could not parse To address %q: %s", to, err)
	}

	for _, groups := range matches {
		if linkUrl, err := url.Parse(groups[1]); err == nil {
			r.output <- UnsubscribeInfo{toAddr, subject, linkUrl}
		} else {
			r.Infof("could not parse link %q: %s", groups[1], err)
		}
	}
}

func (r *MailReader) decodeHeader(headers textproto.MIMEHeader, key string) string {
	value, err := r.decoder.DecodeHeader(headers.Get(key))
	if err != nil {
		value = headers.Get("Subject")
		r.Infof("could not decode value of %q header %q: %s", key, value, err)
	}
	return value
}
