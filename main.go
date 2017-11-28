package main

import (
	"flag"
	"github.com/gmauleon/alertmanager-zabbix-provisioner/provisioner"
	log "github.com/sirupsen/logrus"
	"os"
)

func main() {

	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	//log.SetFormatter(&log.JSONFormatter{})
	log.SetFormatter(&log.TextFormatter{DisableColors: true})

	configFileName := flag.String("config", "./config.yaml", "path to the configuration file")
	flag.Parse()

	cfg, err := provisioner.ConfigFromFile(*configFileName)
	if err != nil {
		log.Fatal(err)
	}

	log.Debug(cfg)

	provisioner.New(cfg).Start()
}
