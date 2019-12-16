package main

import (
	"flag"
	"os"

	"github.com/juju/loggo"
)

func main() {
	_, _ = loggo.ReplaceDefaultWriter(loggo.NewSimpleWriter(os.Stdout, loggo.DefaultFormatter))
	logger := loggo.GetLogger("")
	logger.SetLogLevel(loggo.DEBUG)
	logger.Infof("starting")
	defer logger.Infof("bye bye")

	var configFile string
	flag.StringVar(&configFile, "config", "./unsubscriber.json", "Path of configuration file")
	flag.Parse()

	config, err := LoadConfig(configFile)
	if err != nil {
		logger.Criticalf("could not read configuration: %s", err)
		os.Exit(2)
	}
	if err := loggo.ConfigureLoggers(config.Logs); err != nil {
		logger.Warningf("could not configure logging: %s", err)
	}

	reader, err := NewMailReader(config.IMAP)
	defer func() { _ = reader.Close() }()

	unsubscriber := NewUniqueUnsubscriber(BySchemeUnsubscriber(map[string]Unsubscriber{
		"http":   MakeLoggerUnsubscriber("http"),
		"https":  MakeLoggerUnsubscriber("https"),
		"mailto": MakeLoggerUnsubscriber("mailto"),
	}))

	for info := range reader.C {
		if err := unsubscriber.Unsubscribe(info); err != nil {
			logger.Infof("could not process link: %q", err)
		}
	}
}


