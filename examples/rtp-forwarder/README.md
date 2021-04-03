# rtp-forwarder
rtp-forwarder is a simple application that shows how to forward your webcam/microphone via RTP using Pion WebRTC.

## Instructions
### Download rtp-forwarder
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/rtp-forwarder
```

### Open rtp-forwarder example page
[jsfiddle.net](https://jsfiddle.net/1qva2zd8/) you should see your Webcam, two text-areas and a 'Start Session' button

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
You can run any of these commands at anytime. The media is live/stateless, you can switch commands without restarting Pion.

#### VLC
Open `rtp-forwarder.sdp` with VLC and enjoy your live video!

#### ffmpeg/ffprobe
Run `ffprobe -i rtp-forwarder.sdp -protocol_whitelist file,udp,rtp` to get more details about your streams

Run `ffplay -i rtp-forwarder.sdp -protocol_whitelist file,udp,rtp` to play your streams

You can add `-fflags nobuffer` to lower the latency. You will have worse playback in networks with jitter.

#### Twitch/RTMP
`ffmpeg -protocol_whitelist file,udp,rtp -i rtp-forwarder.sdp -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -ar 44100 -f flv rtmp://live.twitch.tv/app/$STREAM_KEY` Make sure to replace `$STREAM_KEY` at the end of the URL first.
