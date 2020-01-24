// +build linux
package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	done := make(chan bool, 1)

	_, err := os.Stat("config.json")

	if os.IsNotExist(err) {
		log.Println("config.json not found. generating default configuration")
		if err := generateConfig(); err != nil {
			log.Fatal(err)
		}
	}

	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal("could not open file. ", err)
	}

	var config Config

	if err := json.Unmarshal(file, &config); err != nil {
		log.Fatal("could not unmarshal configuration. ", err)
	}

	room := NewRoom()
	go room.Run(done)

	mux := config.Server.Start()
	if err := registerHandlers(mux, room); err != nil {
		log.Fatal(err)
	}

	go config.Camera.EventListener(done, room)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	handleShutdown(done, wg)
	wg.Wait()

	time.Sleep(time.Second)
}

func handleShutdown(done chan bool, wg *sync.WaitGroup) {
	ch := make(chan os.Signal, 1)
	go func() {
		for {
			select {
			case <-done:
				wg.Done()
				return
			case <-ch:
				log.Println("ctrl+c interrupt received")
				close(done)
				wg.Done()
				return
			}
		}
	}()
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
}
