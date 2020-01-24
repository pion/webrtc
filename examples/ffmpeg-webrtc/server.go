package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Server struct {
	Port   string `json:"port"`
	server *http.Server
}

func (s *Server) Start() *mux.Router {
	mux := mux.NewRouter()
	s.server = &http.Server{Addr: s.Port, Handler: mux}

	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			log.Fatal("listen and serve: ", err)
			return
		}
	}()

	return mux
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	if err := s.server.Shutdown(ctx); err != nil {
		log.Fatal("could not stop server.", err)
	}

	cancel()
}
