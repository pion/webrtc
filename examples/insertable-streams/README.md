# insertable-streams
insertable-streams demonstrates how to use insertable streams with Pion.
This example modifies the video with a single-byte XOR cipher before sending, and then
decrypts in Javascript.

insertable-streams allows the browser to process encoded video. You could implement
E2E encyption, add metadata or insert a completely different video feed!

## Instructions
### Create IVF named `output.ivf` that contains a VP8 track
```
ffmpeg -i $INPUT_FILE -g 30 output.ivf
```

### Download insertable-streams
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/insertable-streams
```

### Open insertable-streams example page
[jsfiddle.net](https://jsfiddle.net/t5xoaryc/) you should see two text-areas and a 'Start Session' button. You will also have a 'Decrypt' checkbox.
When unchecked the browser will not decrypt the incoming video stream, so it will stop playing or display certificates.

### Run insertable-streams with your browsers SessionDescription as stdin
The `output.ivf` you created should be in the same directory as `insertable-streams`. In the jsfiddle the top textarea is your browser, copy that and:

#### Linux/macOS
Run `echo $BROWSER_SDP | insertable-streams`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `insertable-streams < my_file`

### Input insertable-streams's SessionDescription into your browser
Copy the text that `insertable-streams` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes. `insertable-streams` will exit when the file reaches the end.

To stop decrypting the stream uncheck the box and the video will not be viewable.

Congrats, you have used Pion WebRTC! Now start building something cool
