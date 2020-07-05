# play-from-disk
play-from-disk demonstrates how to send video and/or audio to your browser from flise saved to disk.

## Instructions
### Create IVF named `output.ivf` that contains a VP8 track and/or `output.ogg` that contains a Opus track
```
ffmpeg -i $INPUT_FILE -g 30 output.ivf
ffmpeg -i $INPUT_FILE -c:a libopus -page_duration 20000 -vn output.ogg
```

### Download play-from-disk
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/play-from-disk
```

### Open play-from-disk example page
[jsfiddle.net](https://jsfiddle.net/y16Ljznr/) you should see two text-areas and a 'Start Session' button

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
