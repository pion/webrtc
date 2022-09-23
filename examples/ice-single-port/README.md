# ice-single-port
ice-single-port demonstrates Pion WebRTC's ability to serve many PeerConnections on a single port.

Pion WebRTC has no global state, so by default ports can't be shared between two PeerConnections.
Using the SettingEngine, a developer can manually share state between many PeerConnections to allow
multiple PeerConnections to use the same port.

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
Open [http://localhost:8080](http://localhost:8080). This will automatically open 10 PeerConnections. This page will print
a Local/Remote line for each PeerConnection. Note that all 10 PeerConnections have different ports for their Local port.
However for the remote they all will be using port 8443.

Congrats, you have used Pion WebRTC! Now start building something cool.
