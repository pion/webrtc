package main

import (
	"sync"

	"github.com/pions/webrtc"
)

type BroadcastHub struct {
	broadcastChannel chan []byte
	listenChannels   map[*uint16]*webrtc.DataChannel
	dataMutex        *sync.RWMutex
}

func newHub() *BroadcastHub {
	hub := &BroadcastHub{
		broadcastChannel: make(chan []byte),
		listenChannels:   make(map[*uint16]*webrtc.DataChannel),
		dataMutex:        new(sync.RWMutex),
	}
	go hub.run()
	return hub
}

func (h *BroadcastHub) addListener(d *webrtc.DataChannel) {
	h.dataMutex.Lock()
	h.listenChannels[d.ID()] = d
	h.dataMutex.Unlock()
}

func (h *BroadcastHub) run() {
	for {
		select {
		case message := <-h.broadcastChannel:
			h.dataMutex.RLock()
			channels := h.listenChannels
			h.dataMutex.RUnlock()
			for _, client := range channels {
				if err := client.SendText(string(message)); err != nil {
					h.dataMutex.Lock()
					delete(h.listenChannels, client.ID())
					h.dataMutex.Unlock()
				}
			}
		}
	}
}
