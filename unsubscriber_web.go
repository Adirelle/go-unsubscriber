package main

import (
	"fmt"
	"github.com/juju/loggo"
	"net/http"
)

type (
	WebUnsubscriber struct {
		loggo.Logger
		Client *http.Client
		HostChecker
	}

	HostChecker interface {
		CheckHost(host string) (bool, error)
	}
)

func (w *WebUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
	if isSafe, err := w.CheckHost(info.Link.Host); err != nil {
		return fmt.Errorf("could not check host %q: %w", info.Link.Host, err)
	} else if !isSafe {
		return fmt.Errorf("unsafe host: %q", info.Link.Host)
	}

	w.Debugf("Sending GET request to %s", info.Link)
	response, err := w.Client.Get(info.Link.String())
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", info.Link, response.Status)
	}
	w.Infof("GET %s: %s", info.Link, response.Status)
	return nil
}
