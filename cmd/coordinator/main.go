package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/cerana/cerana/coordinator"
	logx "github.com/cerana/cerana/pkg/logrusx"
	flag "github.com/spf13/pflag"
)

func main() {
	log.SetFormatter(&logx.MistifyFormatter{})

	config := coordinator.NewConfig(nil, nil)
	flag.Parse()

	dieOnError(config.LoadConfig())
	dieOnError(config.SetupLogging())

	server, err := coordinator.NewServer(config)
	dieOnError(err)

	dieOnError(server.Start())
	server.StopOnSignal()
}

func dieOnError(err error) {
	if err != nil {
		log.Fatal("encountered an error during startup")
	}
}
