package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/juju/loggo"
	"io"
	"net"
	"strings"
)

type (
	ConnectionConfig struct {
		Host     string             `json:"host"`
		Port     uint               `json:"port"`
		Security ConnectionSecurity `json:"security"`
		Login    string             `json:"login"`
		Password Base64Password     `json:"password"`
	}

	ConnectionSecurity struct {
		connectionStrategy
	}

	connectionStrategy interface {
		Connect(addr string, dialer Dialer) (interface{}, error)
		UseSecurePort() bool
		fmt.Stringer
	}

	Dialer func(net.Conn) (interface{}, error)

	PlainTextConnectionStrategy struct{}
	StartTLSConnectionStrategy  struct{ PlainTextConnectionStrategy }
	SSLConnectionStrategy       struct{}

	Base64Password string

	ConnTap struct {
		net.Conn
		Logger loggo.Logger
	}

	Loginer interface {
		Login(string, string) error
	}

	TLSStarter interface {
		StartTLS(*tls.Config) error
	}
)

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

func (c ConnectionConfig) Connect(dialer Dialer) (interface{}, error) {
	conn, err := c.Security.Connect(c.Addr(), dialer)
	if err == nil && c.Login != "" {
		err = conn.(Loginer).Login(c.Login, c.Password.String())
	}
	return conn, err
}

func (s *ConnectionSecurity) UnmarshalText(text []byte) error {
	switch {
	case bytes.EqualFold(text, []byte("plaintext")):
		s.connectionStrategy = PlainTextConnectionStrategy{}
	case bytes.EqualFold(text, []byte("starttls")):
		s.connectionStrategy = StartTLSConnectionStrategy{PlainTextConnectionStrategy{}}
	case bytes.EqualFold(text, []byte("ssl")):
		s.connectionStrategy = SSLConnectionStrategy{}
	default:
		return fmt.Errorf("unknown connection security: %q", text)
	}
	return nil
}

func (PlainTextConnectionStrategy) Connect(addr string, dialer Dialer) (interface{}, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("could not etablish a connection to %s: %w", addr, err)
	}
	return dialer(conn)
}

func (PlainTextConnectionStrategy) UseSecurePort() bool { return false }
func (PlainTextConnectionStrategy) String() string      { return "plainText" }

func (s StartTLSConnectionStrategy) Connect(addr string, dialer Dialer) (interface{}, error) {
	conn, err := s.PlainTextConnectionStrategy.Connect(addr, dialer)
	if err != nil {
		return conn, nil
	}

	if plainTextConn, ok := conn.(TLSStarter); ok {
		if err := plainTextConn.StartTLS(nil); err != nil {
			return nil, fmt.Errorf("could start TLS on connection to %s: %w", addr, err)
		}
	}

	return conn, nil
}

func (StartTLSConnectionStrategy) String() string { return "startTLS" }

func (SSLConnectionStrategy) Connect(addr string, dialer Dialer) (interface{}, error) {
	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("could not etablish a connection to %s: %w", addr, err)
	}
	return dialer(conn)
}

func (S SSLConnectionStrategy) UseSecurePort() bool { return true }
func (S SSLConnectionStrategy) String() string      { return "ssl" }

func (p Base64Password) String() string {
	return string(p)
}

func (p *Base64Password) UnmarshalText(text []byte) error {
	clearPassword, err := base64.StdEncoding.DecodeString(string(text))
	if err == nil {
		*p = Base64Password(clearPassword)
	}
	return err
}

func (c *ConnTap) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err == nil || err == io.EOF {
		c.Logger.Debugf("< %s", bytes.TrimRight(b[0:n], " \n"))
	}
	return
}

func (c *ConnTap) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if err == nil || err == io.EOF {
		c.Logger.Debugf("> %s", bytes.TrimRight(b[0:n], " \n"))
	}
	return
}
