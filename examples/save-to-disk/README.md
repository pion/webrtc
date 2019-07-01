# save-to-disk
save-to-disk is a simple application that shows how to record your webcam using Pion WebRTC and save to disk.

## Instructions
### Download save-to-disk
```
go get github.com/pion/webrtc/examples/save-to-disk
```

### Open save-to-disk example page
[jsfiddle.net](https://jsfiddle.net/b3d72av1/) you should see your Webcam, two text-areas and a 'Start Session' button

### Run save-to-disk, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | save-to-disk`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `save-to-disk < my_file`

### Input save-to-disk's SessionDescription into your browser
Copy the text that `save-to-disk` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, wait, close jsfiddle, enjoy your video!
In the folder you ran `save-to-disk` you should now have a file `output-1.ivf` play with your video player of choice!
> Note: In order to correctly create the files, the remote client (JSFiddle) should be closed. The Go example will automatically close itself.

Congrats, you have used Pion WebRTC! Now start building something cool
