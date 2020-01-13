# play-h264-from-disk
play-h264-from-disk demonstrates how to send h264 video to your browser from a file saved to disk.

WARNING: It doesn't work in Chrome. Only Firefox has been tested successfully.


## Instructions
### Download play-h264-from-disk
```
go get github.com/pion/webrtc/examples/play-h264-from-disk
```

### Create h264 named `output.h264` that contains a H264 track
```
./sample.sh
```

### Open play-h264-from-disk example page
[jsfiddle.net](https://jsfiddle.net/234b95ja/) you should see two text-areas and a 'Start Session' button

### Run play-h264-from-disk with your browsers SessionDescription as stdin
The `output.h264` you created should be in the same directory as `play-h264-from-disk`. In the jsfiddle the top textarea is your browser, copy that and:

#### Linux/macOS
Run `echo $BROWSER_SDP | play-h264-from-disk`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `play-h264-from-disk < my_file`

### Input play-h264-from-disk's SessionDescription into your browser
Copy the text that `play-h264-from-disk` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes. `play-h264-from-disk` will exit when the file reaches the end

Congrats, you have used Pion WebRTC! Now start building something cool
