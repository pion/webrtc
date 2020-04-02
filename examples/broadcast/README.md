# broadcast
broadcast is a Pion WebRTC application that demonstrates how to broadcast a video to many peers, while only requiring the broadcaster to upload once.

This could serve as the building block to building conferencing software, and other applications where publishers are bandwidth constrained.

## Instructions
### Download broadcast
```
go get github.com/pion/webrtc/v2/examples/broadcast
```

### Open broadcast example page
[jsfiddle.net](https://jsfiddle.net/d2mt13y6/) You should see two buttons 'Publish a Broadcast' and 'Join a Broadcast'

### Run Broadcast
#### Linux/macOS
Run `broadcast` OR run `main.go` in `github.com/pion/webrtc/examples/broadcast`

### Start a publisher

* Click `Publish a Broadcast`
* `curl localhost:8080/sdp -d "YOUR SDP"`.  The `broadcast` application will respond with an offer, paste this into the second input field. Then press `Start Session`

### Join the broadcast
* Click `Join a Broadcast`
* `curl localhost:8080/sdp -d "YOUR SDP"`. The `broadcast` application will respond with an offer, paste this into the second input field. Then press `Start Session`

You can change the listening port using `-port 8011`

You can `Join the broadcast` as many times as you want. The `broadcast` Golang application is relaying all traffic, so your browser only has to upload once.

Congrats, you have used Pion WebRTC! Now start building something cool
