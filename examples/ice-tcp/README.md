# ice-tcp
ice-tcp demonstrates Pion WebRTC's ICE TCP abilities.

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
