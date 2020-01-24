package main

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type Room struct {
	Join    chan *Client
	Leave   chan *Client
	Clients map[*Client]bool
	//Event handling comm chans
	Inbound  chan *Message
	Outbound chan *Message
}

type Message struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Message interface{} `json:"message"`
}

func NewRoom() *Room {
	return &Room{
		Join:     make(chan *Client, 1),
		Leave:    make(chan *Client, 1),
		Clients:  make(map[*Client]bool),
		Inbound:  make(chan *Message, 1),
		Outbound: make(chan *Message, 1),
	}
}

func (w *Room) Run(done chan bool) {
	for {
		select {
		case <-done:
			log.Println("stopping websocket server")
			return
		case client := <-w.Join:
			w.Clients[client] = true

		case client := <-w.Leave:
			delete(w.Clients, client)
			close(client.Inbound)

		//server to all clients
		case msg := <-w.Outbound:
			if len(w.Clients) > 0 {
				for client := range w.Clients {
					m, err := json.Marshal(msg)
					if err != nil {
						log.Println("wshub could not marshal we message.", err)
						continue
					}
					client.Inbound <- m
				}
			}
		}
	}
}

type Client struct {
	Socket  *websocket.Conn
	Inbound chan []byte
	Room    *Room
}

//browser to server
func (c *Client) Read() {
	defer c.Socket.Close()

	for {
		_, msg, err := c.Socket.ReadMessage()
		if err != nil {
			return
		}

		var m *Message

		if err := json.Unmarshal(msg, &m); err != nil {
			log.Println("client could not unmarshal ws message.", err)
			continue
		}

		c.Room.Inbound <- m
	}
}

//server to browser
func (c *Client) Write() {
	defer c.Socket.Close()
	for msg := range c.Inbound {
		err := c.Socket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
		}
	}
}
