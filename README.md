<h1 align="center">
  <a href="https://pion.ly"><img src="./.github/pion-gopher-webrtc.png" alt="Pion WebRTC" height="250px"></a>
  <br>
  Pion WebRTC
  <br>
</h1>
<h4 align="center">A pure Go implementation of the WebRTC API</h4>
<p align="center">
  <a href="https://pion.ly"><img src="https://img.shields.io/badge/pion-webrtc-gray.svg?longCache=true&colorB=brightgreen" alt="Pion webrtc"></a>
  <a href="https://sourcegraph.com/github.com/pion/webrtc?badge"><img src="https://sourcegraph.com/github.com/pion/webrtc/-/badge.svg" alt="Sourcegraph Widget"></a>
  <a href="https://pion.ly/slack"><img src="https://img.shields.io/badge/join-us%20on%20slack-gray.svg?longCache=true&logo=slack&colorB=brightgreen" alt="Slack Widget"></a>
  <br>
  <a href="https://travis-ci.org/pion/webrtc"><img src="https://travis-ci.org/pion/webrtc.svg?branch=master" alt="Build Status"></a>
  <a href="https://godoc.org/github.com/pion/webrtc"><img src="https://godoc.org/github.com/pion/webrtc?status.svg" alt="GoDoc"></a>
  <a href="https://codecov.io/gh/pion/webrtc"><img src="https://codecov.io/gh/pion/webrtc/branch/master/graph/badge.svg" alt="Coverage Status"></a>
  <a href="https://goreportcard.com/report/github.com/pion/webrtc"><img src="https://goreportcard.com/badge/github.com/pion/webrtc" alt="Go Report Card"></a>
  <a href="https://www.codacy.com/app/Sean-Der/webrtc"><img src="https://api.codacy.com/project/badge/Grade/18f4aec384894e6aac0b94effe51961d" alt="Codacy Badge"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>
<br>


See [DESIGN.md](DESIGN.md) for an overview of features and future goals.

### Breaking Changes

Pion WebRTC v2.0.0 has arrived! See the [release notes](https://pion.ly/knowledge-base/release-notes/webrtc-v2.0.0/) to learn about new features and breaking changes.

Have any questions? Join [the Slack channel](https://pion.ly/slack) to follow development and speak with the maintainers.

We are actively planning [v2.1.0](https://github.com/pion/webrtc/projects/11) and would love your feedback! Anyone can add issues, and anything that you think can empower Pion users.

### Usage
Check out the **[example applications](examples/README.md)** to help you along your Pion WebRTC journey.

For more full featured examples that use 3rd party libraries see our **[example-webrtc-applications](https://github.com/pion/example-webrtc-applications)** repo.

The Pion WebRTC API closely matches the JavaScript **[WebRTC API](https://w3c.github.io/webrtc-pc/)**. Most existing documentation is therefore also usefull when working with Pion. Furthermore, our **[GoDoc](https://godoc.org/github.com/pion/webrtc)** is actively maintained.

We maintain a [FAQ](https://pion.ly/knowledge-base/pion-basics/faq/) with answers to common questions. If you have a question not covered please submit a PR, we would be happy to answer it!

Now go forth and build some awesome apps! Here are some **ideas** to get your creative juices flowing:
* Send a video file to multiple browser in real time for perfectly synchronized movie watching.
* Send a webcam on an embedded device to your browser with no additional server required!
* Securely send data between two servers, without using pub/sub.
* Record your webcam and do special effects server side.
* Build a conferencing application that processes audio/video and make decisions off of it.

### WebAssembly
Pion WebRTC can be used when compiled to WebAssembly, also known as Wasm. In
this case the library will act as a wrapper around the JavaScript WebRTC API.
This allows you to use WebRTC from Go in both server and browser side code with
little to no changes. Check out the
**[example applications](examples/README.md#webassembly)** for instructions on
how to compile and run the WebAssembly examples. You can also visit the
[Wiki page on WebAssembly Development](https://github.com/pion/webrtc/wiki/WebAssembly-Development-and-Testing)
for more information.

### Roadmap
The library is in active development, please refer to the [roadmap](https://github.com/pion/webrtc/issues/9) to track our major milestones.

### Community
Pion has an active community on the [Golang Slack](https://invite.slack.golangbridge.org/). Sign up and join the **#pion** channel for discussions and support. You can also use [Pion mailing list](https://groups.google.com/forum/#!forum/pion).

We are always looking to support **your projects**. Please reach out if you have something to build!

If you need commercial support or don't want to use public methods you can contact us at [team@pion.ly](mailto:team@pion.ly)


### Project status

[![Stargazers over time](https://starchart.cc/pion/webrtc.svg)](https://starchart.cc/pion/webrtc)



### Related projects
* [pion/turn](https://github.com/pion/turn): A simple extendable Golang TURN server
* [WIP] [pion/media-server](https://github.com/pion/media-server): A Pion WebRTC powered media server, providing the building blocks for anything RTC.

### Contributing
Check out the **[contributing wiki](https://github.com/pion/webrtc/wiki/Contributing)** to join the group of amazing people making this project possible:

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
* [Denis](https://github.com/Hixon10) - *Adding docker-compose to pion-to-pion example*
* [earle](https://github.com/aguilEA) - *Generate DTLS fingerprint in Go*
* [Jake B](https://github.com/silbinarywolf) - *Fix Windows installation instructions*
* [Michael MacDonald](https://github.com/mjmac) - *Plan B compatibility, Remote TURN/Trickle-ICE, Logging framework*
* [Oleg Kovalov](https://github.com/cristaloleg) *Use wildcards instead of hardcoding travis-ci config*
* [Woodrow Douglass](https://github.com/wdouglass) *RTCP, RTP improvements, G.722 support, Bugfixes*
* [Tobias Fridén](https://github.com/tobiasfriden) *SRTP authentication verification*
* [Yutaka Takeda](https://github.com/enobufs) *Fix ICE connection timeout*
* [Hugo Arregui](https://github.com/hugoArregui) *Fix connection timeout*
* [Rob Deutsch](https://github.com/rob-deutsch) *RTPReceiver graceful shutdown*
* [Jin Lei](https://github.com/jinleileiking) - *SFU example use http*
* [Will Watson](https://github.com/wwatson) - *Enable gocritic*
* [Luke Curley](https://github.com/kixelated)
* [Antoine Baché](https://github.com/Antonito) - *OGG Opus export*
* [frank](https://github.com/feixiao) - *Building examples on OSX*
* [mxmCherry](https://github.com/mxmCherry)
* [Alex Browne](https://github.com/albrow) - *JavaScript/Wasm bindings*
* [adwpc](https://github.com/adwpc) - *SFU example with websocket*
* [imalic3](https://github.com/imalic3) - *SFU websocket example with datachannel broadcast*
* [Žiga Željko](https://github.com/zigazeljko)
* [Simonacca Fotokite](https://github.com/simonacca-fotokite)
* [Marouane](https://github.com/nindolabs) *Fix Offer bundle generation*
* [Christopher Fry](https://github.com/christopherfry)
* [Adam Kiss](https://github.com/masterada)
* [xsbchen](https://github.com/xsbchen)
* [Alex Harford](https://github.com/alexjh)
* [Aleksandr Razumov](https://github.com/ernado)
* [mchlrhw](https://github.com/mchlrhw)
* [AlexWoo(武杰)](https://github.com/AlexWoo) *Fix RemoteDescription parsing for certificate fingerprint*
* [Cecylia Bocovich](https://github.com/cohosh)
* [Slugalisk](https://github.com/slugalisk)
* [Agugua Kenechukwu](https://github.com/spaceCh1mp)
* [Ato Araki](https://github.com/atotto)
* [Rafael Viscarra](https://github.com/rviscarra)
* [Mike Coleman](https://github.com/fivebats)
* [Suhas Gaddam](https://github.com/suhasgaddam)
* [Atsushi Watanabe](https://github.com/at-wat)
* [Robert Eperjesi](https://github.com/epes)
* [Aaron France](https://github.com/AeroNotix)
* [Gareth Hayes](https://github.com/gazhayes)
* [Sebastian Waisbrot](https://github.com/seppo0010)
* [Masataka Hisasue](https://github.com/sylba2050) - *Fix Docs*
* [Hongchao Ma(马洪超)](https://github.com/hcm007)
* [Aaron France](https://github.com/AeroNotix)
* [Chris Hiszpanski](https://github.com/thinkski) - *Fix Answer bundle generation*

### License
MIT License - see [LICENSE](LICENSE) for full text
