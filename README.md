# Pion WebRTC
[![Build Status](https://travis-ci.org/pions/webrtc.svg?branch=master)](https://travis-ci.org/pions/webrtc)
[![GoDoc](https://godoc.org/github.com/pions/webrtc?status.svg)](https://godoc.org/github.com/pions/webrtc)
[![Go Report Card](https://goreportcard.com/badge/github.com/pions/webrtc)](https://goreportcard.com/report/github.com/pions/webrtc)
[![Coverage Status](https://coveralls.io/repos/github/pions/webrtc/badge.svg)](https://coveralls.io/github/pions/webrtc)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/18f4aec384894e6aac0b94effe51961d)](https://www.codacy.com/app/Sean-Der/webrtc)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.md)

A (almost) pure Golang implementation of the WebRTC API.

See [DESIGN.md](DESIGN.md) for the the features it offers, and future goals.

## Getting Started
This project provides a Go implementation of the WebRTC API. There isn't a application that will fit all your needs, but we provide a
few simple examples to show common use cases (Sending and Receiving video) that you are free to modify and extend to your needs

### Example Programs
#### gstreamer
GStreamer shows how to use pion-WebRTC with GStreamer. Once connected it streams video to an autovideosink

#### save-to-disk
save-to-disk shows how to use pion-WebRTC to record and save video. Once connected it saves video to *.ivf files which can be played with ffmpeg

### Writing your own application
The example applications provide examples for common use cases. The API should match the Javascript WebRTC API, and the [GoDoc](https://godoc.org/github.com/pions/webrtc) is actively maintained

## Roadmap
pion-WebRTC is in active development, you can find the roadmap [here](https://github.com/pions/webrtc/issues/9).

The master branch will always be usable, but will have features that aren't completed.

## Questions/Support
Sign up for the [Golang Slack](https://invite.slack.golangbridge.org/) and join the #pion channel for discussions and support

If you need commercial support/don't want to use public methods you can contact us at [team@pion.ly](team@pion.ly)

## Project Ideas
I am looking to support other interesting WebRTC projects, so if you have something to build please reach out!
pion-WebRTC would make a great foundation for.

* Easy language bindings (Python)
* Golang SFU
* Server side processing (video effects or an MCU)

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md)

### Contributors

* [John Bradley](https://github.com/kc5nra) - *Original Author*
* [Sean DuBois](https://github.com/Sean-Der) - *Original Author*

## License
MIT License - see [LICENSE.md](LICENSE.md) for full text
