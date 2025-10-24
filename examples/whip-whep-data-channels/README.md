# whip-whep
whip-whep demonstrates using WHIP and WHEP with Pion. Since WHIP+WHEP is standardized signaling you can publish via tools like OBS and GStreamer.
You can then watch it in sub-second time from your browser, or pull the video back into OBS and GStreamer via WHEP.

Further details about the why and how of WHIP+WHEP are below the instructions.

## Instructions

### Download whip-whep

This example requires you to clone the repo since it is serving static HTML.

```
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/whip-whep
```

### Run whip-whep
Execute `go run *.go`

### Publish

You can publish via an tool that supports WHIP or via your browser. To publish via your browser open [http://localhost:8080](http://localhost:8080), and press publish.

To publish via OBS set `Service` to `WHIP` and `Server` to `http://localhost:8080/whip`. The `Bearer Token` can be whatever value you like.


### Subscribe

Once you have started publishing open [http://localhost:8080](http://localhost:8080) and press the subscribe button. You can now view your video you published via
OBS or your browser.

Congrats, you have used Pion WebRTC! Now start building something cool

## Why WHIP/WHEP?

WHIP/WHEP mandates that a Offer is uploaded via HTTP. The server responds with a Answer. With this strong API contract WebRTC support can be added to tools like OBS.

For more info on WHIP/WHEP specification, feel free to read some of these great resources:
- https://webrtchacks.com/webrtc-cracks-the-whip-on-obs/
- https://datatracker.ietf.org/doc/draft-ietf-wish-whip/
- https://datatracker.ietf.org/doc/draft-ietf-wish-whep/
- https://bloggeek.me/whip-whep-webrtc-live-streaming
