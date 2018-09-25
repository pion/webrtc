<h1 align="center">
  <a href="https://pion.ly"><img src="./.github/pion-gopher-webrtc.png" alt="Pion WebRTC" height="250px"></a>
  <br>
  Pion WebRTC
  <br>
</h1>
<h4 align="center">A Golang implementation of the WebRTC API</h4>
<br>
<p align="center">
  <a href="https://travis-ci.org/pions/webrtc"><img src="https://travis-ci.org/pions/webrtc.svg?branch=master" alt="Build Status"></a>
  <a href="https://godoc.org/github.com/pions/webrtc"><img src="https://godoc.org/github.com/pions/webrtc?status.svg" alt="GoDoc"></a>
  <a href="https://goreportcard.com/report/github.com/pions/webrtc"><img src="https://goreportcard.com/badge/github.com/pions/webrtc" alt="Go Report Card"></a>
  <a href="https://coveralls.io/github/pions/webrtc"><img src="https://coveralls.io/repos/github/pions/webrtc/badge.svg" alt="Coverage Status"></a>
  <a href="https://www.codacy.com/app/Sean-Der/webrtc"><img src="https://api.codacy.com/project/badge/Grade/18f4aec384894e6aac0b94effe51961d" alt="Codacy Badge"></a>
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>
<br>

See [DESIGN.md](DESIGN.md) for an overview of features and future goals.

### Install
Currently Pion uses CGO and OpenSSL for DTLS. We are actively working on replacing it. For now you have to make sure to install a C compliler and the OpenSSL headers for
your platform:
#### Ubuntu/Debian
`sudo apt-get install libssl-dev`
#### OSX
```
brew install openssl
export CPATH=`brew --prefix`/opt/openssl/include
export LIBRARY_PATH=`brew --prefix`/opt/openssl/lib
go get -u github.com/pions/webrtc
```
#### Fedora
`sudo yum install openssl-devel`
#### Windows
1. Install [mingw-w64](http://mingw-w64.sourceforge.net/)
2. Install [pkg-config-lite](http://sourceforge.net/projects/pkgconfiglite)
3. Build (or install precompiled) openssl for mingw32-w64
4. Set __PKG\_CONFIG\_PATH__ to the directory containing openssl.pc
   (i.e. c:\mingw64\mingw64\lib\pkgconfig)

### Usage
Check out the **[example applications](examples/README.md)** to help you along your Pion WebRTC journey.

The Pion WebRTC API closely matches the JavaScript **[WebRTC API](https://w3c.github.io/webrtc-pc/)**. Most existing documentation is therefore also usefull when working with Pion. Furthermore, our **[GoDoc](https://godoc.org/github.com/pions/webrtc)** is actively maintained.

Now go forth and build some awesome apps! Here are some **ideas** to get your creative juices flowing:
* Send a video file to multiple browser in real time for perfectly synchronized movie watching.
* Send a webcam on an embedded device to your browser with no additional server required!
* Securely send data between two servers, without using pub/sub.
* Record your webcam and do special effects server side.
* Build a conferencing application that processes audio/video and make decisions off of it.

### Roadmap
The library is in active development, please refer to the [roadmap](https://github.com/pions/webrtc/issues/9) to track our major milestones.

### Community
Pion has an active community on the [Golang Slack](https://invite.slack.golangbridge.org/). Sign up and join the **#pion** channel for discussions and support. You can also use [Pion mailing list](https://groups.google.com/forum/#!forum/pion).

We are always looking to support **your projects**. Please reach out if you have something to build!

If you need commercial support or don't want to use public methods you can contact us at [team@pion.ly](mailto:team@pion.ly)

### Related projects
* [pions/turn](https://github.com/pions/turn): A simple extendable Golang TURN server
* [WIP] [pions/media-server](https://github.com/pions/media-server): A Pion WebRTC powered media server, providing the building blocks for anything RTC.
* [WIP] [pions/dcnet](https://github.com/pions/dcnet): A package providing Golang [net](https://godoc.org/net) interfaces around Pion WebRTC data channels.

### Contributing
Check out the **[contributing wiki](https://github.com/pions/webrtc/wiki/Contributing)** to join the group of amazing people making this project possible:

* [John Bradley](https://github.com/kc5nra) - *Original Author*
* [Michael Melvin Santry](https://github.com/santrym) - *Mascot*
* [Raphael Randschau](https://github.com/nicolai86) - *STUN*
* [Sean DuBois](https://github.com/Sean-Der) - *Original Author*
* [Michiel De Backker](https://github.com/backkem) - *SDP, Public API, Project Management*
* [Brendan Rius](https://github.com/brendanrius) - *Cleanup*
* [Konstantin Itskov](https://github.com/trivigy) - *SDP Parsing*
* [chenkaiC4](https://github.com/chenkaiC4) - *Fix GolangCI Linter*
* [Ronan J](https://github.com/ronanj) - *Fix STCP PPID*
* [wattanakorn495](https://github.com/wattanakorn495)
* [Max Hawkins](https://github.com/maxhawkins) - *RTCP*
* [Justin Okamoto](https://github.com/justinokamoto) - *Fix Docs*
* [leeoxiang](https://github.com/notedit) - *Implement Janus examples*

### License
MIT License - see [LICENSE.md](LICENSE.md) for full text
