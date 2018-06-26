# Pion WebRTC
[![Build Status](https://travis-ci.org/pions/webrtc.svg?branch=master)](https://travis-ci.org/pions/webrtc)
[![GoDoc](https://godoc.org/github.com/pions/webrtc?status.svg)](https://godoc.org/github.com/pions/webrtc)
[![Go Report Card](https://goreportcard.com/badge/github.com/pions/webrtc)](https://goreportcard.com/report/github.com/pions/webrtc)
[![Coverage Status](https://coveralls.io/repos/github/pions/webrtc/badge.svg)](https://coveralls.io/github/pions/webrtc)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/18f4aec384894e6aac0b94effe51961d)](https://www.codacy.com/app/Sean-Der/webrtc)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.md)

<div align="center">
    <a href="#">
        <img src="./.github/pion-gopher-webrtc.png" height="300px">
    </a>
</div>

A Golang implementation of the WebRTC API.

See [DESIGN.md](DESIGN.md) for the features it offers, and future goals.

## Getting Started
This project provides a Go implementation of the WebRTC API. There isn't a application that will fit all your needs, but we provide a
few simple examples to show common use cases that you are free to modify and extend to your needs.

### What can I build with pion-WebRTC?
pion-WebRTC is here to help you get media/text from A<->B, here are some of the cool things you could build.

* Send a video file to multiple browser in real time, perfectly synchronized movie watching.
* Send a webcam on a small device to your browser, with no additional server required
* Securely send video between two servers
* Record your webcam and do special effects server side
* Build a conferencing application that processes audio/video and make decisions off of it

### Prerequisites
We still use OpenSSL for DTLS (we are actively working on replacing it) so make sure to install the OpenSSL headers for
your platform before using pion-WebRTC.
#### Ubuntu/Debian
`sudo apt-get install libssl-dev`

#### OSX
`brew install openssl`

#### Fedora
`sudo yum install openssl-devel`

#### Windows
1. Install [mingw-w64](http://mingw-w64.sourceforge.net/)
2. Install [pkg-config-lite](http://sourceforge.net/projects/pkgconfiglite)
3. Build (or install precompiled) openssl for mingw32-w64
4. Set __PKG\_CONFIG\_PATH__ to the directory containing openssl.pc
   (i.e. c:\mingw64\mingw64\lib\pkgconfig)

### Example Programs
Examples for common use cases, extend and modify to quickly get started.
* [gstreamer-receive](examples/gstreamer-receive/README.md) Play video from your Webcam live using GStreamer
* [save-to-disk](examples/save-to-disk/README.md) Save video from your Webcam to disk

### Writing your own application
The API should match the Javascript WebRTC API, and the [GoDoc](https://godoc.org/github.com/pions/webrtc) is actively maintained

## Roadmap
pion-WebRTC is in active development, you can find the roadmap [here](https://github.com/pions/webrtc/issues/9).

## Questions/Support
Sign up for the [Golang Slack](https://invite.slack.golangbridge.org/) and join the #pion channel for discussions and support

You can also use [Pion mailing list](https://groups.google.com/forum/#!forum/pion)

If you need commercial support/don't want to use public methods you can contact us at [team@pion.ly](mailto:team@pion.ly)

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md)

### Contributors

* [John Bradley](https://github.com/kc5nra) - *Original Author*
* [Sean DuBois](https://github.com/Sean-Der) - *Original Author*
* [Michael Melvin Santry](https://github.com/santrym) - *Mascot*

## Project Ideas
I am looking to support other interesting WebRTC projects, so if you have something to build please reach out!
pion-WebRTC would make a great foundation for.

* Easy language bindings (Python)
* Golang SFU
* Server side processing (video effects or an MCU)

## License
MIT License - see [LICENSE.md](LICENSE.md) for full text
