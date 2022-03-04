# rtp-to-webrtc
rtp-to-webrtc demonstrates how to consume a RTP stream video UDP, and then send to a WebRTC client.

With this example we have pre-made GStreamer and ffmpeg pipelines, but you can use any tool you like!

## Instructions
### Download rtp-to-webrtc
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/rtp-to-webrtc
```

### Open jsfiddle example page
[jsfiddle.net](https://jsfiddle.net/z7ms3u5r/) you should see two text-areas and a 'Start Session' button


### Run rtp-to-webrtc with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser's SessionDescription, copy that and:

#### Linux/macOS
Run `echo $BROWSER_SDP | rtp-to-webrtc`

#### Windows
1. Paste the SessionDescription into a file.
1. Run `rtp-to-webrtc < my_file`

### Send RTP to listening socket
You can use any software to send VP8 packets to port 5004. We also have the pre made examples below


#### GStreamer
```
gst-launch-1.0 videotestsrc ! video/x-raw,width=640,height=480,format=I420 ! vp8enc error-resilient=partitions keyframe-max-dist=10 auto-alt-ref=true cpu-used=5 deadline=1 ! rtpvp8pay ! udpsink host=127.0.0.1 port=5004
```

#### ffmpeg
```
ffmpeg -re -f lavfi -i testsrc=size=640x480:rate=30 -vcodec libvpx -cpu-used 5 -deadline 1 -g 10 -error-resilient 1 -auto-alt-ref 1 -f rtp 'rtp://127.0.0.1:5004?pkt_size=1200'
```

If you wish to send audio replace all occurrences of `vp8` with Opus in `main.go` then run

```
ffmpeg -f lavfi -i 'sine=frequency=1000' -c:a libopus -b:a 48000 -sample_fmt s16p -ssrc 1 -payload_type 111 -f rtp -max_delay 0 -application lowdelay 'rtp://127.0.0.1:5004?pkt_size=1200'
```

If you wish to send H264 instead of VP8 replace all occurrences of `vp8` with H264 in `main.go` then run

```
ffmpeg -re -f lavfi -i testsrc=size=640x480:rate=30 -pix_fmt yuv420p -c:v libx264 -g 10 -preset ultrafast -tune zerolatency -f rtp 'rtp://127.0.0.1:5004?pkt_size=1200'
```

### Input rtp-to-webrtc's SessionDescription into your browser
Copy the text that `rtp-to-webrtc` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes.

Congrats, you have used Pion WebRTC! Now start building something cool

## Dealing with broken/lossy inputs
Pion WebRTC also provides a [SampleBuilder](https://pkg.go.dev/github.com/pion/webrtc/v3@v3.0.4/pkg/media/samplebuilder). This consumes RTP packets and returns samples.
It can be used to re-order and delay for lossy streams. You can see its usage in this example in [daf27b](https://github.com/pion/webrtc/commit/daf27bd0598233b57428b7809587ec3c09510413).

Currently it isn't working with H264, but is useful for VP8 and Opus. See [#1652](https://github.com/pion/webrtc/issues/1652) for the status of fixing for H264.
