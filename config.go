package main

import (
	"encoding/json"
	"fmt"
	"github.com/juju/loggo"
	"os"
)

type (
	IMAPConfig struct {
		ConnectionConfig
		Mailbox string `json:"mailbox"`
	}

	Config struct {
		IMAP IMAPConfig       `json:"imap"`
		SMTP ConnectionConfig `json:"smtp"`
		Logs string           `json:"logs"`
	}
)

func (c IMAPConfig) String() string {
	return fmt.Sprintf("{%s}%s", c.ConnectionConfig, c.Mailbox)
}

func LoadConfig(path string) (Config, error) {
	var conf Config

	if reader, err := os.Open(path); err == nil {
		defer func() { _ = reader.Close() }()
		loggo.GetLogger("config").Infof("reading configuration from %s", path)
		err := json.NewDecoder(reader).Decode(&conf)
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
