# sfu
sfu is a pion-WebRTC application that demonstrates how to broadcast a video to many peers, while only requiring the broadcaster to upload once.

This could serve as the building block to building conferencing software, and other applications where publishers are bandwidth constrained.

## Instructions
### Download sfu
```
go get github.com/pions/webrtc/examples/sfu
```

### Open sfu example page
[jsfiddle.net](https://jsfiddle.net/5cwx0rns/11/) You should see two buttons 'Publish a Broadcast' and 'Join a Broadcast'

### Run SFU
#### Linux/macOS
Run `sfu` OR run `main.go` in `github.com/pions/webrtc/examples/sfu`

### Start a publisher

* Click `Publish a Broadcast`
* `curl localhost:8080/sdp -d "YOUR SDP"`.  The `sfu` application will respond with an offer, paste this into the second input field. Then press `Start Session`

### Join the broadcast
* Click `Join a Broadcast`
* `curl localhost:8080/sdp -d "YOUR SDP"`. The `sfu` application will respond with an offer, paste this into the second input field. Then press `Start Session`

You can change the listening port using `-port 8011`

You can `Join the broadcast` as many times as you want. The `sfu` Golang application is relaying all traffic, so your browser only has to upload once.

Congrats, you have used pion-WebRTC! Now start building something cool
