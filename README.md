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
  <a href="https://github.com/pion/awesome-pion" alt="Awesome Pion"><img src="https://cdn.rawgit.com/sindresorhus/awesome/d7305f38d29fed78fa85652e3a63e154dd8e8829/media/badge.svg"></a>
  <br>
  <a href="https://travis-ci.org/pion/webrtc"><img src="https://travis-ci.org/pion/webrtc.svg?branch=master" alt="Build Status"></a>
  <a href="https://pkg.go.dev/github.com/pion/webrtc/v2"><img src="https://godoc.org/github.com/pion/webrtc?status.svg" alt="GoDoc"></a>
  <a href="https://codecov.io/gh/pion/webrtc"><img src="https://codecov.io/gh/pion/webrtc/branch/master/graph/badge.svg" alt="Coverage Status"></a>
  <a href="https://goreportcard.com/report/github.com/pion/webrtc"><img src="https://goreportcard.com/badge/github.com/pion/webrtc" alt="Go Report Card"></a>
  <a href="https://www.codacy.com/app/Sean-Der/webrtc"><img src="https://api.codacy.com/project/badge/Grade/18f4aec384894e6aac0b94effe51961d" alt="Codacy Badge"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</p>
<br>

Pion WebRTC is a pure Go implementation of WebRTC. It has zero non-Go dependencies and no 3rd party Go dependencies. It is designed to follow **[WebRTC API](https://w3c.github.io/webrtc-pc/)**, but may deviate when required.
See [DESIGN.md](DESIGN.md) for the guiding principals/inspirations of the project.

### Usage
**[example applications](examples/README.md)** contains code samples of common things people build with Pion WebRTC.

**[example-webrtc-applications](https://github.com/pion/example-webrtc-applications)** contains more full featured examples that use 3rd party libraries.

**[awesome-pion](https://github.com/pion/awesome-pion)** contains projects that have used Pion, and serve as real world examples of usage.

**[GoDoc](https://godoc.org/github.com/pion/webrtc)** is an auto generated API reference. All our Public APIs are commented.

**[FAQ](https://github.com/pion/webrtc/wiki/FAQ)** has answers to common questions. If you have a question not covered please ask in [Slack](https://pion.ly/slack) we are always looking to expand it.

Now go build something awesome! Here are some **ideas** to get your creative juices flowing:
* Send a video file to multiple browser in real time for perfectly synchronized movie watching.
* Send a webcam on an embedded device to your browser with no additional server required!
* Securely send data between two servers, without using pub/sub.
* Record your webcam and do special effects server side.
* Build a conferencing application that processes audio/video and make decisions off of it.

### WebAssembly
Pion WebRTC can be used when compiled to WebAssembly, also known as WASM. In
this case the library will act as a wrapper around the JavaScript WebRTC API.
This allows you to use WebRTC from Go in both server and browser side code with
little to no changes. Check out the
**[example applications](examples/README.md#webassembly)** for instructions on
how to compile and run the WebAssembly examples. You can also visit the
[Wiki page on WebAssembly Development](https://github.com/pion/webrtc/wiki/WebAssembly-Development-and-Testing)
for more information.

### Roadmap
The library is in active development, please refer to the [roadmap](https://github.com/pion/webrtc/issues/9) to track our major milestones.
We also maintain a list of [Big Ideas](https://github.com/pion/webrtc/wiki/Big-Ideas) these are things we want to build but don't have a clear plan or the resources yet.
If you are looking to get involved this is a great place to get started! We would also love to hear your ideas! Even if you can't implement it yourself, it could inspire others.

### Community
Pion has an active community on the [Slack](https://pion.ly/slack).

Follow the [Pion Twitter](https://twitter.com/_pion) for project updates and important WebRTC news.

We are always looking to support **your projects**. Please reach out if you have something to build!
If you need commercial support or don't want to use public methods you can contact us at [team@pion.ly](mailto:team@pion.ly)

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
* [Vicken Simonian](https://github.com/vsimon)
* [Guilherme Souza](https://github.com/gqgs)
* [Andrew N. Shalaev](https://github.com/isqad)
* [David Hamilton](https://github.com/dihamilton)
* [Ilya Mayorov](https://github.com/faroyam)
* [Patrick Lange](https://github.com/langep)
* [cyannuk](https://github.com/cyannuk)
* [Lukas Herman](https://github.com/lherman-cs)
* [Konstantin Chugalinskiy](https://github.com/kchugalinskiy)
* [Bao Nguyen](https://github.com/sysbot)
* [Luke S](https://github.com/encounter)
* [Hendrik Hofstadt](https://github.com/hendrikhofstadt)
* [Clayton McCray](https://github.com/ClaytonMcCray)
* [lawl](https://github.com/lawl)
* [Jorropo](https://github.com/Jorropo)
* [Akil](https://github.com/akilude)
* [Quentin Renard](https://github.com/asticode)
* [opennota](https://github.com/opennota)
* [Simon Eisenmann](https://github.com/longsleep)
* [Ben Weitzman](https://github.com/benweitzman)
* [Masahiro Nakamura](https://github.com/tsuu32)
* [Tarrence van As](https://github.com/tarrencev)
* [Yuki Igarashi](https://github.com/bonprosoft)

### License
MIT License - see [LICENSE](LICENSE) for full text
