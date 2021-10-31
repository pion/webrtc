# ice-tcp
ice-tcp demonstrates Pion WebRTC's ICE TCP abilities.

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Fice-tcp%2Fdrawio)

## Instructions

### Download ice-tcp
This example requires you to clone the repo since it is serving static HTML.

```
mkdir -p $GOPATH/src/github.com/pion
cd $GOPATH/src/github.com/pion
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/ice-tcp
```

### Run ice-tcp
Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080). This will automatically start a PeerConnection. This page will now prints stats about the PeerConnection. The UDP candidates will be filtered out from the SDP.

Congrats, you have used Pion WebRTC! Now start building something cool
