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
	}
)

func (w *WebUnsubscriber) Unsubscribe(info UnsubscribeInfo) error {
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
