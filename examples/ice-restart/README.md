# ice-restart
ice-restart demonstrates Pion WebRTC's ICE Restart abilities.

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Fice-restart%2Fdrawio)

## Instructions

### Download ice-restart
This example requires you to clone the repo since it is serving static HTML.

```
mkdir -p $GOPATH/src/github.com/pion
cd $GOPATH/src/github.com/pion
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/ice-restart
```

### Run ice-restart
Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080). This will automatically start a PeerConnection. This page will now prints stats about the PeerConnection
and allow you to do an ICE Restart at anytime.

* `ICE Restart` is the button that causes a new offer to be made wih `iceRestart: true`.
* `ICE Connection States` will contain all the connection states the PeerConnection moves through.
* `ICE Selected Pairs` will print the selected pair every 3 seconds. Note how the uFrag/uPwd/Port change everytime you start the Restart process.
* `Inbound DataChannel Messages` containing the current time sent by the Pion process every 3 seconds.

Congrats, you have used Pion WebRTC! Now start building something cool
