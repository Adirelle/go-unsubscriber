package main

import (
	"fmt"
	"github.com/juju/loggo"
)

type (
	Unsubscriber interface {
		Unsubscribe(info UnsubscribeInfo) error
	}

	UniqueUnsubscriber struct {
		next Unsubscriber
		seen map[string]bool
	}

	BySchemeUnsubscriber map[string]Unsubscriber

	LoggerUnsubscriber struct {
		loggo.Logger
	}
)

func (b BySchemeUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	if next, ok := b[info.Link.Scheme]; ok {
		return next.Unsubscribe(info)
	}
	return fmt.Errorf("scheme %q is not supported", info.Link.Scheme)
}

func MakeLoggerUnsubscriber(name string) LoggerUnsubscriber {
	return LoggerUnsubscriber{loggo.GetLogger("unsubscriber." + name)}
}

func (l LoggerUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	l.Infof("unsubscribing %#+v", info.Link)
	return nil
}

func NewUniqueUnsubscriber(next Unsubscriber) UniqueUnsubscriber {
	return UniqueUnsubscriber{next, make(map[string]bool, 50)}
}

func (u UniqueUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	key := info.Link.Host
	if info.Link.Scheme == "mailto" {
		key = info.Link.Opaque
	}
	if u.seen[key] {
		return fmt.Errorf("%q already processed", key)
	}
	u.seen[key] = true
	return u.next.Unsubscribe(info)
}
