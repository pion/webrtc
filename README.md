# Pion WebRTC
[![GoDoc](https://godoc.org/github.com/pions/turn?status.svg)](https://godoc.org/github.com/pions/turn)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.md)

A (almost) pure Golang implementation of the WebRTC Native API.

# Status
Things need to be completed before it is usable for public consumption.
- [x] ICE-lite (peers can communicate directly via host candidates)
- [x] DTLS
- [x] SRTP
- [x] API that matches WebRTC spec

Things that I plan to do, but will happen only when someone requests/I need it.
* [ ] Native DTLS (Currently we use OpenSSL)
* [ ] Native SRTP (Currently we use libsrtp2)
* [ ] DataChannels
* [ ] TURN/STUN/ICE
* [ ] Sending Video

# How to use
This project provides an API to work with WebRTC clients. To see a quick demo we provide an example.

Build (or run) [save-to-disk](https://github.com/pions/webrtc/tree/master/examples/save-to-disk) This will first ask for your browsers SDP, and then provide yours

Open the [demo page](https://jsfiddle.net/tr2uq31e/1/) and get the base64 from the browser, then provide ours and press 'Start Session'

The Go application should print when it gets any packets, and the web page will print it's status as well.

# Project Ideas
I am looking to support other interesting WebRTC projects, so if you have something to build please reach out!
pion-WebRTC would make a great foundation for.

* Easy language bindings (Python)
* Golang SFU
* Server side processing (video effects or an MCU)
