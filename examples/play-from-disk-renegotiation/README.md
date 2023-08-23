# play-from-disk-renegotiation
play-from-disk-renegotiation demonstrates Pion WebRTC's renegotiation abilities.

For a simpler example of playing a file from disk we also have [examples/play-from-disk](/examples/play-from-disk)

## Instructions

### Download play-from-disk-renegotiation
This example requires you to clone the repo since it is serving static HTML.

```
mkdir -p $GOPATH/src/github.com/pion
cd $GOPATH/src/github.com/pion
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/play-from-disk-renegotiation
```

### Create IVF named `output.ivf` that contains a VP8 track

```
ffmpeg -i $INPUT_FILE -g 30 -b:v 2M output.ivf
```

**Note**: In the `ffmpeg` command, the argument `-b:v 2M` specifies the video bitrate to be 2 megabits per second. We provide this default value to produce decent video quality, but if you experience problems with this configuration (such as dropped frames etc.), you can decrease this. See the [ffmpeg documentation](https://ffmpeg.org/ffmpeg.html#Options) for more information on the format of the value.

### Run play-from-disk-renegotiation

The `output.ivf` you created should be in the same directory as `play-from-disk-renegotiation`. Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080) and you should have a `Add Track` and `Remove Track` button.  Press these to add as many tracks as you want, or to remove as many as you wish.

Congrats, you have used Pion WebRTC! Now start building something cool
