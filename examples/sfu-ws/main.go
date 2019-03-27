package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/pions/webrtc"
	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func init() {

	// Generate pem file for https
	genPem()

	// Create a MediaEngine object to configure the supported codec
	m = webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	m.RegisterCodec(webrtc.NewRTPOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000))

	// Create the API object with the MediaEngine
	api = webrtc.NewAPI(webrtc.WithMediaEngine(m))

}

func main() {

	prometheus.Register(prommod.NewCollector("sfu-ws"))

	port := flag.String("p", "8443", "https port")
	flag.Parse()

	http.Handle("/metrics", promhttp.Handler())

	// Websocket handle func
	http.HandleFunc("/ws", room)

	// Html handle func
	http.HandleFunc("/", web)

	// Support https, so we can test by lan
	fmt.Println("Web listening :" + *port)
	panic(http.ListenAndServeTLS(":"+*port, "cert.pem", "key.pem", nil))
}
