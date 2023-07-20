# save-to-disk-av1
save-to-disk-av1 is a simple application that shows how to save a video to disk using AV1.

If you wish to save VP8 and Opus instead of AV1 see [save-to-disk](https://github.com/pion/webrtc/tree/master/examples/save-to-disk)

If you wish to save VP8/Opus inside the same file see [save-to-webm](https://github.com/pion/example-webrtc-applications/tree/master/save-to-webm)

You can then send this video back to your browser using [play-from-disk](https://github.com/pion/example-webrtc-applications/tree/master/play-from-disk)

## Instructions
### Download save-to-disk-av1
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/save-to-disk-av1
```

### Open save-to-disk-av1 example page
[jsfiddle.net](https://jsfiddle.net/xjcve6d3/) you should see your Webcam, two text-areas and two buttons: `Copy browser SDP to clipboard`, `Start Session`.

### Run save-to-disk-av1, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser's Session Description. Press `Copy browser SDP to clipboard` or copy the base64 string manually.
We will use this value in the next step.

#### Linux/macOS
Run `echo $BROWSER_SDP | save-to-disk-av1`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `save-to-disk-av1 < my_file`

### Input save-to-disk-av1's SessionDescription into your browser
Copy the text that `save-to-disk-av1` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, wait, close jsfiddle, enjoy your video!
In the folder you ran `save-to-disk-av1` you should now have a file `output.ivf` play with your video player of choice!
> Note: In order to correctly create the files, the remote client (JSFiddle) should be closed. The Go example will automatically close itself.

Congrats, you have used Pion WebRTC! Now start building something cool
