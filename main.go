package main

import (
	"flag"
	"github.com/gmauleon/alertmanager-zabbix-provisioner/provisioner"
	"github.com/sirupsen/logrus"
	"os"
)

var log = logrus.WithField("context", "main")

func main() {

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{DisableColors: true})

	configFileName := flag.String("config", "./config.yaml", "path to the configuration file")
	flag.Parse()

	cfg, err := provisioner.ConfigFromFile(*configFileName)
	if err != nil {
		log.Fatal(err)
	}

	provisioner.New(cfg).Start()
}
