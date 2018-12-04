# janus-gateway
janus-gateway is a collection of examples showing how to use pion-WebRTC with [janus-gateway](https://github.com/meetecho/janus-gateway)

These examples require that you build+enable websockets with Janus

## streaming
This example demonstrates how to download a video from a Janus streaming room. Before you run this example, you need to run `plugins/streams/test_gstreamer_1.sh` from Janus.

You should confirm that you can successfully watch `Opus/VP8 live stream coming from gstreamer (live)` in the stream demo web UI

### Running
run `main.go` in `github.com/pions/webrtc/examples/janus-gateway/streaming`

If this worked you will see the following.
```
Connection State has changed Checking
Connection State has changed Connected
Got VP8 track, saving to disk as output.ivf
```

You will see output.ivf in the current folder.
