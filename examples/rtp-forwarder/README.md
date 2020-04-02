# rtp-forwarder
rtp-forwarder is a simple application that shows how to forward your webcam/microphone via RTP using Pion WebRTC.

## Instructions
### Download rtp-forwarder
```
go get github.com/pion/webrtc/v2/examples/rtp-forwarder
```

### Open rtp-forwarder example page
[jsfiddle.net](https://jsfiddle.net/sq69370h/) you should see your Webcam, two text-areas and a 'Start Session' button

### Run rtp-forwarder, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | rtp-forwarder`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `rtp-forwarder < my_file`

### Input rtp-forwarder's SessionDescription into your browser
Copy the text that `rtp-forwarder` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle and enjoy your RTP forwarded stream!
#### VLC
Open `rtp-forwarder.sdp` with VLC and enjoy your live video!

### ffmpeg/ffprobe
Run `ffprobe -i rtp-forwarder.sdp -protocol_whitelist file,udp,rtp` to get more details about your streams

Run `ffplay -i rtp-forwarder.sdp -protocol_whitelist file,udp,rtp` to play your streams

