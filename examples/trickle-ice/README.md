# trickle-ice
trickle-ice demonstrates Pion WebRTC's Trickle ICE APIs.  ICE is the subsystem WebRTC uses to establish connectivity.

Trickle ICE is the process of sharing addresses as soon as they are gathered. This parallelizes
establishing a connection with a remote peer and starting sessions with TURN servers. Using Trickle ICE
can dramatically reduce the amount of time it takes to establish a WebRTC connection.

Trickle ICE isn't mandatory to use, but highly recommended.

## Instructions

### Download trickle-ice
This example requires you to clone the repo since it is serving static HTML.

```
mkdir -p $GOPATH/src/github.com/pion
cd $GOPATH/src/github.com/pion
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/trickle-ice
```

### Run trickle-ice
Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080). This will automatically start a PeerConnection.

## Note
Congrats, you have used Pion WebRTC! Now start building something cool
