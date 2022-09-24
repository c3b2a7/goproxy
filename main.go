package main

import (
	"fmt"
	"github.com/c3b2a7/goproxy/services"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/signal"
	"syscall"
)

var (
	app     *kingpin.Application
	service *services.ServiceItem
)

func main() {
	err := initConfig()
	if err != nil {
		fmt.Printf("[-] Error: %s\n", err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	if service != nil {
		fmt.Printf("\n[*] Received an interrupt, stopping services...\n")
		service.S.Clean()
	}
	KillSubProcess()
}
