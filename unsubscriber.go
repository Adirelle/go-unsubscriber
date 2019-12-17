package main

import (
	"errors"
	"fmt"
	"github.com/juju/loggo"
	"sync"
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

	ConcurrentUnsubscriber struct {
		next  Unsubscriber
		tasks chan UnsubscribeInfo
		done  sync.WaitGroup
		loggo.Logger
	}
)

var (
	ErrDuplicate = errors.New("already processed")
)

func NewUniqueUnsubscriber(next Unsubscriber) UniqueUnsubscriber {
	return UniqueUnsubscriber{next, make(map[string]bool, 50)}
}

func (u UniqueUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	key := info.Link.Host
	if info.Link.Scheme == "mailto" {
		key = info.Link.Opaque
	}
	if u.seen[key] {
		return fmt.Errorf("%q: %w", key, ErrDuplicate)
	}
	u.seen[key] = true
	return u.next.Unsubscribe(info)
}

func (b BySchemeUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	if next, ok := b[info.Link.Scheme]; ok {
		return next.Unsubscribe(info)
	}
	return fmt.Errorf("scheme %q is not supported", info.Link.Scheme)
}

func NewConcurrentUnsubscriber(next Unsubscriber, maxConcurrent int) *ConcurrentUnsubscriber {
	c := &ConcurrentUnsubscriber{
		next:   next,
		tasks:  make(chan UnsubscribeInfo),
		Logger: loggo.GetLogger("unsubscriber.concurrent"),
	}

	c.done.Add(maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		go c.run(i)
	}

	return c
}

func (c *ConcurrentUnsubscriber) run(i int) {
	defer func() {
		c.Debugf("worker %d ended", i)
		c.done.Done()
	}()
	c.Debugf("worker %d started", i)

	for info := range c.tasks {
		if err := c.next.Unsubscribe(info); err != nil {
			c.Infof("error handling link: %s", err)
		}
	}
}

func (c *ConcurrentUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	c.tasks <- info
	return nil
}

func (c *ConcurrentUnsubscriber) Close() error {
	c.Debugf("closing")
	close(c.tasks)
	c.done.Wait()
	c.Debugf("done")
	return nil
}
