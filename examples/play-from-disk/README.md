# play-from-disk
play-from-disk demonstrates how to send video and/or audio to your browser from files saved to disk.

For an example of playing H264 from disk see [play-from-disk-h264](https://github.com/pion/example-webrtc-applications/tree/master/play-from-disk-h264)

## (architecture)[https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Fplay-from-disk%2Fdrawio]



## Instructions
### Create IVF named `output.ivf` that contains a VP8 track and/or `output.ogg` that contains a Opus track
```
ffmpeg -i $INPUT_FILE -g 30 -b:v 2M output.ivf
ffmpeg -i $INPUT_FILE -c:a libopus -page_duration 20000 -vn output.ogg
```

**Note**: In the `ffmpeg` command which produces the .ivf file, the argument `-b:v 2M` specifies the video bitrate to be 2 megabits per second. We provide this default value to produce decent video quality, but if you experience problems with this configuration (such as dropped frames etc.), you can decrease this. See the [ffmpeg documentation](https://ffmpeg.org/ffmpeg.html#Options) for more information on the format of the value.

### Download play-from-disk

```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/play-from-disk
```

### Open play-from-disk example page
[jsfiddle.net](https://jsfiddle.net/9s10amwL/) you should see two text-areas and a 'Start Session' button

### Run play-from-disk with your browsers SessionDescription as stdin
The `output.ivf` you created should be in the same directory as `play-from-disk`. In the jsfiddle the top textarea is your browser, copy that and:

#### Linux/macOS
Run `echo $BROWSER_SDP | play-from-disk`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `play-from-disk < my_file`

### Input play-from-disk's SessionDescription into your browser
Copy the text that `play-from-disk` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes. `play-from-disk` will exit when the file reaches the end

Congrats, you have used Pion WebRTC! Now start building something cool
