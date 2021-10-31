# ice-single-port
ice-single-port demonstrates Pion WebRTC's ability to serve many PeerConnections on a single port.

Pion WebRTC has no global state, so by default ports can't be shared between two PeerConnections.
Using the SettingEngine a developer can manually share state between many PeerConnections and allow
multiple to use the same port

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Fice-single-port%2Fdrawio)

## Instructions

### Download ice-single-port
This example requires you to clone the repo since it is serving static HTML.

```
mkdir -p $GOPATH/src/github.com/pion
cd $GOPATH/src/github.com/pion
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/ice-single-port
```

### Run ice-single-port
Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080). This will automatically open 5 PeerConnections. This page will print
a Local/Remote line for each PeerConnection. Note that all 10 PeerConnections have different ports for their Local port.
However for the remote they all will be using port 8443.

Congrats, you have used Pion WebRTC! Now start building something cool
