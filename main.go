package main

import (
	"fmt"
	"github.com/c3b2a7/goproxy/services"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	err := initConfig()
	if err != nil {
		log.Fatalf("err : %s", err)
	}
	Clean(&service.S)
}
func Clean(s *services.Service) {
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for _ = range signalChan {
			fmt.Println("\nReceived an interrupt, stopping services...")
			(*s).Clean()
			cleanupDone <- true
		}
	}()
	<-cleanupDone
}
