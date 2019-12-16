package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/sirupsen/logrus"
	"mime"
	"net/textproto"
)

const (
	UnsubscribedFlag = "unsubscribed"
)

type (
	UnsubscribeInfo struct {
		To        string
		Subject   string
		Links     []string
		MessageId string
	}

	MailReader struct {
		C <-chan UnsubscribeInfo

		client *client.Client
		output chan UnsubscribeInfo
		*logrus.Entry
		decoder mime.WordDecoder
	}
)

var headerSectionName *imap.BodySectionName

func init() {
	var err error
	headerSectionName, err = imap.ParseBodySectionName("BODY.PEEK[HEADER]")
	if err != nil {
		panic(err.Error())
	}
}

func NewMailReader(config IMAPConfig) (r *MailReader, err error) {
	r = new(MailReader)
	r.Entry = logrus.WithField("imap", config.String())
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
		r.Debug("closing connection")
		return r.client.Logout()
	}
	return nil
}

func (r *MailReader) connect(config IMAPConfig) error {
	if conn, err := config.Connect(
		func(addr string) (interface{}, error) {
			return client.Dial(addr)
		},
		func(addr string, c *tls.Config) (interface{}, error) {
			return client.DialTLS(addr, c)
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
	criteria.Header.Set("List-Unsubscribe", "")
	criteria.WithoutFlags = []string{imap.DeletedFlag}

	r.Debugf("searching messages with these criteria: %s", criteria.Format())
	uids, err := r.client.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("could not search for messages: %w", err)
	}
	r.Infof("found %d messages", len(uids))

	var seqSet imap.SeqSet
	seqSet.AddNum(uids...)

	return &seqSet, nil
}

func (r *MailReader) fetchMails(uids *imap.SeqSet) {
	r.Debug("fetching messages")
	messages := make(chan *imap.Message, 10)

	go func() {
		defer close(r.output)
		for message := range messages {
			r.processMessage(message)
		}
	}()

	err := r.client.UidFetch(uids, []imap.FetchItem{headerSectionName.FetchItem(), imap.FetchFlags}, messages)
	if err != nil {
		r.Warnf("could not fetch (all) mail(s): %s", err)
	}

	r.Info("all messages processed")
}

func (r *MailReader) processMessage(message *imap.Message) {
	l := r.WithField("uid", message.Uid)
	l.Debugf("received message")

	if info, err := r.parseMessage(message); err == nil {
		r.output <- *info
	} else {
		l.Infof("ignoring message because of: %s", err)
	}
}

func (r *MailReader) parseMessage(message *imap.Message) (*UnsubscribeInfo, error) {
	reader := textproto.NewReader(bufio.NewReader(message.GetBody(headerSectionName)))
	headers, err := reader.ReadMIMEHeader()
	if err != nil {
		return nil, fmt.Errorf("could not parse MIME headers: %w", err)
	}

	to, err := r.decoder.DecodeHeader(headers.Get("To"))
	if err != nil {
		to = headers.Get("To")
		r.Infof("could not decode To header %q: %s", to, err)
	}

	subject, err := r.decoder.DecodeHeader(headers.Get("Subject"))
	if err != nil {
		subject = headers.Get("Subject")
		r.Infof("could not decode Subject header %q: %s", subject, err)
	}

	return &UnsubscribeInfo{to,subject,headers["List-Unsubscribe"],headers.Get("Message-ID")}, nil
}
