package main

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	gorilla "github.com/gorilla/websocket"
)

func handleIndex(t *template.Template) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		templ := t.Lookup("index.html")

		host := r.Host

		if err := templ.Execute(w, host); err != nil {
			log.Println("failed to execute template.", err)
		}
	})
}

func handleWS(room *Room) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			socketBufferSize  = 1024
			messageBufferSize = 256
		)

		var upgrader = &gorilla.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}

		socket, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal("ServeHTTP", err)
			return
		}

		client := &Client{
			Socket:  socket,
			Inbound: make(chan []byte, messageBufferSize),
			Room:    room,
		}

		room.Join <- client

		defer func() {
			room.Leave <- client
		}()

		go client.Write()
		client.Read()
	})
}

func registerHandlers(gmux *mux.Router, room *Room) error {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return err
	}

	fileNames, err := fetchTemplateFiles(filepath.Dir(p) + "/assets/templates/")

	if err != nil {
		return err
	}

	templates := template.New("templates")

	template.Must(templates.ParseFiles(fileNames...))

	gmux.Handle("/", handleIndex(templates))

	fs := http.FileServer(http.Dir("."))
	gmux.PathPrefix("/assets/").Handler(fs)

	gmux.Handle("/ws", handleWS(room))

	return nil
}

func fetchTemplateFiles(path string) ([]string, error) {
	fileNames := []string{}

	err := filepath.Walk(path, func(currentPath string, file os.FileInfo, err error) error {
		if file.IsDir() {
			files, err := ioutil.ReadDir(currentPath)
			if err != nil {
				return errors.New(fmt.Sprint("fetchTemplateFiles could not read directory. ", err))
			}

			for _, f := range files {
				if f.IsDir() {
					continue
				}
				fileNames = append(fileNames, currentPath+"/"+f.Name())
			}
		}
		return nil
	})

	return fileNames, err
}
