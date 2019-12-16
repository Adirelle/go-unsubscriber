package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/juju/loggo"
	"os"
	"strings"
)

type (
	ConnectionSecurity byte

	ConnectionConfig struct {
		Host     string             `json:"host"`
		Port     uint               `json:"port"`
		Security ConnectionSecurity `json:"security"`
		Login    string             `json:"login"`
		Password string             `json:"password"`
	}

	IMAPConfig struct {
		ConnectionConfig
		Mailbox string `json:"mailbox"`
	}

	Config struct {
		IMAP IMAPConfig       `json:"imap"`
		SMTP ConnectionConfig `json:"smtp"`
		Logs string           `json:"logs"`
	}

	PlainTextDialer func(string) (interface{}, error)
	TLSDialer       func(string, *tls.Config) (interface{}, error)
	TLSStarter      interface {
		StartTLS(*tls.Config) error
	}
	Loginer interface {
		Login(string, string) error
	}
)

const (
	PlainText ConnectionSecurity = 0
	StartTLS  ConnectionSecurity = 1
	SSL       ConnectionSecurity = 2
)

func (s *ConnectionSecurity) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "plaintext":
		*s = PlainText
	case "tls":
		*s = StartTLS
	case "ssl":
		*s = SSL
	default:
		return errors.New("unknown connection security: " + str)
	}
	return nil
}

func (s ConnectionSecurity) UseSecurePort() bool {
	return s == SSL
}

func (s ConnectionSecurity) String() string {
	switch s {
	case PlainText:
		return "plaintext"
	case StartTLS:
		return "tls"
	case SSL:
		return "ssl"
	}
	panic(fmt.Sprintf("unknown connection security: %d", s))
}

func (s ConnectionSecurity) Connect(addr string, dialer PlainTextDialer, tlsDialer TLSDialer) (interface{}, error) {
	switch s {
	case PlainText:
		return dialer(addr)
	case StartTLS:
		conn, err := dialer(addr)
		if err != nil {
			return nil, err
		}
		return conn, conn.(TLSStarter).StartTLS(nil)
	case SSL:
		return tlsDialer(addr, nil)
	}
	panic(fmt.Sprintf("unknown connection security: %d", s))
}

func (c ConnectionConfig) String() string {
	b := &strings.Builder{}
	if c.Login != "" {
		_, _ = b.WriteString(c.Login)
		if c.Password != "" {
			_, _ = b.WriteString(":xxx")
		}
		_, _ = b.WriteString("@")
	}
	_, _ = fmt.Fprintf(b, "%s:%d/%s", c.Host, c.Port, c.Security)
	return b.String()
}

func (c ConnectionConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c ConnectionConfig) Connect(dialer PlainTextDialer, tlsDialer TLSDialer) (interface{}, error) {
	conn, err := c.Security.Connect(c.Addr(), dialer, tlsDialer)
	if err == nil && c.Login != "" {
		err = conn.(Loginer).Login(c.Login, c.Password)
	}
	return conn, err
}

func (c IMAPConfig) String() string {
	return fmt.Sprintf("{%s}%s", c.ConnectionConfig, c.Mailbox)
}

func LoadConfig(path string) (Config, error) {
	var conf Config

	if reader, err := os.Open(path); err == nil {
		loggo.GetLogger("config").Infof("reading configuration from %s", path)
		err := json.NewDecoder(reader).Decode(&conf)
		_ = reader.Close()
		if err != nil {
			return conf, err
		}

	} else if !os.IsNotExist(err) {
		return conf, err
	}

	if conf.IMAP.Port == 0 {
		if conf.IMAP.Security.UseSecurePort() {
			conf.IMAP.Port = 993
		} else {
			conf.IMAP.Port = 143
		}
	}

	if conf.IMAP.Mailbox == "" {
		conf.IMAP.Mailbox = "INBOX"
	}

	if conf.SMTP.Port == 0 {
		if conf.SMTP.Security.UseSecurePort() {
			conf.SMTP.Port = 465
		} else {
			conf.SMTP.Port = 25
		}
	}

	return conf, nil
}
