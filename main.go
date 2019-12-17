package main

import (
	"flag"
	"github.com/juju/loggo"
	"github.com/juju/loggo/loggocolor"
	"os"
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


