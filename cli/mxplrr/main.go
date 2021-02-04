package main

import (
	log "github.com/sirupsen/logrus"
	"mxplrr/cmd"
	"os"
)

func main() {
	log.SetOutput(os.Stdout)

	log.SetFormatter(&log.TextFormatter{
		ForceColors: true,
	})

	cmd.Execute()
}
