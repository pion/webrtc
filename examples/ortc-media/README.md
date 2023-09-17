# ortc-media
ortc demonstrates Pion WebRTC's [ORTC](https://ortc.org/) capabilities. Instead of using the Session Description Protocol
to configure and communicate ORTC provides APIs. Users then can implement signaling with whatever protocol they wish.
ORTC can then be used to implement WebRTC. A ORTC implementation can parse/emit Session Description and act as a WebRTC
implementation.

In this example we have defined a simple JSON based signaling protocol.

## Instructions
### Create IVF named `output.ivf` that contains a VP8/VP9/AV1 track
```
ffmpeg -i $INPUT_FILE -g 30 -b:v 2M output.ivf
```

**Note**: In the `ffmpeg` command which produces the .ivf file, the argument `-b:v 2M` specifies the video bitrate to be 2 megabits per second. We provide this default value to produce decent video quality, but if you experience problems with this configuration (such as dropped frames etc.), you can decrease this. See the [ffmpeg documentation](https://ffmpeg.org/ffmpeg.html#Options) for more information on the format of the value.


### Download ortc-media
```
go install github.com/pion/webrtc/v4/examples/ortc-media@latest
```

### Run first client as offerer
`ortc-media -offer` this will emit a base64 message. Copy this message to your clipboard.

## Run the second client as answerer
Run the second client. This should be launched with the message you copied in the previous step as stdin.

`echo BASE64_MESSAGE_YOU_COPIED | ortc-media`

This will emit another base64 message. Copy this new message.

## Send base64 message to first client via CURL

* Run `curl localhost:8080 -d "BASE64_MESSAGE_YOU_COPIED"`. `BASE64_MESSAGE_YOU_COPIED` is the value you copied in the last step.

### Enjoy
The client that accepts media will print when it gets the first media packet. The SSRC will be different every run.

```
Got RTP Packet with SSRC 3097857772
```

Media packets will continue to flow until the end of the file has been reached.

Congrats, you have used Pion WebRTC! Now start building something cool
