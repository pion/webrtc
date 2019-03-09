# sfu-ws
sfu-ws is a pion-WebRTC application that demonstrates how to broadcast a video to many peers, while only requiring the broadcaster to upload once.

This could serve as the building block to building conferencing software, and other applications where publishers are bandwidth constrained.

## Instructions
### Download sfu-ws
```
go get github.com/pions/webrtc/examples/sfu-ws
```

### Run SFU
#### Linux/macOS
go build
./sfu-ws

### Start a publisher

* Click `Publish`

### Start a Subscriber
* Click `Subscribe`


You can start one publisher and many subscriber

Congrats, you have used pion-WebRTC! Now start building something cool
