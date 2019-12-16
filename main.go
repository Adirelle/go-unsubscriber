package main

import (
	"flag"

	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	logrus.SetOutput(colorable.NewColorableStdout())

	var configFile string
	flag.StringVar(&configFile, "config", "./unsubscriber.json", "Path of configuration file")
	flag.Parse()

	config, err := LoadConfig(configFile)
	if err != nil {
		logrus.Fatalf("could not read configuration: %s", err)
	}

	reader, err := NewMailReader(config.IMAP)
	defer func() { _ = reader.Close() }()

	for info := range reader.C {
		logrus.WithField("message", info).Debug("got message")
	}

	logrus.Info("bye bye")
}
