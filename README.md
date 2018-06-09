# Pion WebRTC
[![GoDoc](https://godoc.org/github.com/pions/turn?status.svg)](https://godoc.org/github.com/pions/turn)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.md)

A (almost) pure Golang implementation of the WebRTC Native API.

# Status
Things need to be completed before it is usable for public consumption.
- [x] ICE-lite (peers can communicate directly via host candidates)
- [x] DTLS
- [x] SRTP
- [ ] API that matches WebRTC spec

Things that I plan to do, but will happen only when someone requests/I need it.
* [ ] Native DTLS (Currently we use OpenSSL)
* [ ] Native SRTP (Currently we use libsrtp2)
* [ ] DataChannels
* [ ] TURN/STUN/ICE
* [ ] Sending Video

# How to use
Build (or run) this project `go run *.go` This will print your SDP, and a base64 version of it.

Open the [demo page](https://jsfiddle.net/tr2uq31e/) and put the base64
from running the Go application in the text area, and press 'Start Session'

The Go application should print when it gets any packets, and the web page will print it's status as well.

# Project Ideas
I am looking to support other interesting WebRTC projects, so if you have something to build please reach out!
pion-WebRTC would make a great foundation for.

* Easy language bindings (Python)
* Golang SFU
* Server side processing (video effects or an MCU)
