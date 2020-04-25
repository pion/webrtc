# rtp2webrtc
rtp2webrtc demonstrates how to send video to your browser from a `rtp`.

## Instructions
```
gst-launch-1.0 videotestsrc ! 'video/x-raw, width=640, height=480' ! videoconvert ! video/x-raw,format=I420 ! vp8enc error-resilient=partitions keyframe-max-dist=10 auto-alt-ref=true cpu-used=5 deadline=1 ! rtpvp8pay ! udpsink host=127.0.0.1 port=5004
```

### Download rtp2webrtc
```
go get github.com/pion/webrtc/v2/examples/rtp2webrtc
```

### Open play-from-disk example page
**Use play-from-disk** [jsfiddle.net](https://jsfiddle.net/z7ms3u5r/) you should see two text-areas and a 'Start Session' button

#### Linux/macOS
Run `echo $BROWSER_SDP | rtp2webrtc`

#### Windows
1. Paste the SessionDescription into a file.
1. Run `rtp2webrtc < my_file`

### Input rtp2webrtc's SessionDescription into your browser
Copy the text that `rtp2webrtc` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes. `rtp2webrtc` will exit when the file reaches the end

Congrats, you have used Pion WebRTC! Now start building something cool
