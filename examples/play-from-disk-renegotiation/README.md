# play-from-disk-renegotiation
play-from-disk-renegotiation demonstrates Pion WebRTC's renegotiation abilities.

For a simpler example of playing a file from disk we also have [examples/play-from-disk](/examples/play-from-disk)

## Instructions

### Download play-from-disk-renegotiation
This example requires you to clone the repo since it is serving static HTML.

```
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/play-from-disk-renegotiation
```

### Create IVF named `output.ivf` that contains a VP8, VP9 or AV1 track

To encode video to VP8:
```
ffmpeg -i $INPUT_FILE -c:v libvpx -g 30 -b:v 2M output.ivf
```

alternatively, to encode video to AV1 (Note: AV1 is CPU intensive, you may need to adjust `-cpu-used`):
```
ffmpeg -i $INPUT_FILE -c:v libaom-av1 -cpu-used 8 -g 30 -b:v 2M output.ivf
```

Or to encode video to VP9:
```
ffmpeg -i $INPUT_FILE -c:v libvpx-vp9 -cpu-used 4 -g 30 -b:v 2M output.ivf
```

If you have a VP8, VP9 or AV1 file in a different container you can use `ffmpeg` to mux it into IVF:
```
ffmpeg -i $INPUT_FILE -c:v copy -an output.ivf
```

**Note**: In the `ffmpeg` command, the argument `-b:v 2M` specifies the video bitrate to be 2 megabits per second. We provide this default value to produce decent video quality, but if you experience problems with this configuration (such as dropped frames etc.), you can decrease this. See the [ffmpeg documentation](https://ffmpeg.org/ffmpeg.html#Options) for more information on the format of the value.

### Run play-from-disk-renegotiation

The `output.ivf` you created should be in the same directory as `play-from-disk-renegotiation`. Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080) and you should have a `Add Track` and `Remove Track` button.  Press these to add as many tracks as you want, or to remove as many as you wish.

Congrats, you have used Pion WebRTC! Now start building something cool
