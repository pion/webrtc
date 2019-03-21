package main

import "github.com/pions/webrtc"

type BroadcastHub struct {
	broadcastChannel chan []byte
	listenChannels   map[*uint16]*webrtc.DataChannel
}

func newHub() *BroadcastHub {
	hub := &BroadcastHub{
		broadcastChannel: make(chan []byte),
		listenChannels:   make(map[*uint16]*webrtc.DataChannel),
	}
	go hub.run()
	return hub
}

func (h *BroadcastHub) addListener(d *webrtc.DataChannel) {
	h.listenChannels[d.ID] = d
}

func (h *BroadcastHub) run() {
	for {
		select {
		case message := <-h.broadcastChannel:
			for _, client := range h.listenChannels {
				if err := client.SendText(string(message)); err != nil {
					delete(h.listenChannels, client.ID)
				}
			}
		}
	}
}
