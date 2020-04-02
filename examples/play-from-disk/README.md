# play-from-disk
play-from-disk demonstrates how to send video to your browser from a file saved to disk.

## Instructions
### Create IVF named `output.ivf` that contains a VP8 track
```
ffmpeg -i $INPUT_FILE -g 30 output.ivf
```

### Download play-from-disk
```
go get github.com/pion/webrtc/v2/examples/play-from-disk
```

### Open play-from-disk example page
[jsfiddle.net](https://jsfiddle.net/z7ms3u5r/) you should see two text-areas and a 'Start Session' button

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
