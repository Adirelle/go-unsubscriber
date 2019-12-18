package main

import (
	"errors"
	"flag"
	"github.com/juju/loggo"
	"github.com/juju/loggo/loggocolor"
	"net/http"
	"os"
	"time"
)

func main() {
	configFile := flag.String("config", "./unsubscriber.json", "Path of configuration file")
	enableDebug := flag.Bool("debug", false, "Display debug messages")
	quiet := flag.Bool("quiet", false, "Only display errors")
	verbose := flag.Bool("verbose", false, "Display more messages")
	colors := flag.Bool("colors", true, "Enable colored output")
	flag.Parse()

	logger := loggo.GetLogger("")
	logger.SetLogLevel(loggo.WARNING)
	if *colors {
		_, _ = loggo.ReplaceDefaultWriter(loggocolor.NewColorWriter(os.Stdout))
	}

	config, err := LoadConfig(*configFile)
	if err != nil {
		logger.Criticalf("could not read configuration: %s", err)
		os.Exit(2)
	}
	if err := loggo.ConfigureLoggers(config.Logs); err != nil {
		logger.Warningf("could not configure logging: %s", err)
	}

	if *enableDebug {
		logger.SetLogLevel(loggo.DEBUG)
	} else if *verbose {
		logger.SetLogLevel(loggo.INFO)
	} else if *quiet {
		logger.SetLogLevel(loggo.ERROR)
	}
	logger.Infof("starting")
	defer logger.Infof("bye bye")

	reader, err := NewMailReader(config.IMAP)
	if err != nil {
		logger.Criticalf("could not initialise the IMAP reader: %w", err)
		os.Exit(1)
	}
	defer func() { _ = reader.Close() }()

	netChecker, err := NewNonLocalAddressChecker()
	if err != nil {
		logger.Criticalf("could not initialise the host checker: %w", err)
		os.Exit(1)
	}

	webUnsubscriber := &WebUnsubscriber{
		loggo.GetLogger("unsubscriber.web"),
		&http.Client{Timeout: 10 * time.Second},
		netChecker,
	}

	concurrentUnsubscriber := NewConcurrentUnsubscriber(
		BySchemeUnsubscriber(map[string]Unsubscriber{
			"http":  webUnsubscriber,
			"https": webUnsubscriber,
		}),
		5,
	)
	defer func() { _ = concurrentUnsubscriber.Close() }()

	unsubscriber := NewUniqueUnsubscriber(concurrentUnsubscriber)
	for info := range reader.C {
		if err := unsubscriber.Unsubscribe(info); err != nil && !errors.Is(err, ErrDuplicate) {
			logger.Infof("could not process link: %q", err)
		}
	}
}
