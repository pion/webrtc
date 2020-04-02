# reflect
reflect demonstrates how with one PeerConnection you can send video to Pion and have the packets sent back. This example could be easily extended to do server side processing.

## Instructions
### Download reflect
```
go get github.com/pion/webrtc/v2/examples/reflect
```

### Open reflect example page
[jsfiddle.net](https://jsfiddle.net/j3yhron4/) you should see two text-areas and a 'Start Session' button.

### Run reflect, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | reflect`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `reflect < my_file`

### Input reflect's SessionDescription into your browser
Copy the text that `reflect` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
Your browser should send video to Pion, and then it will be relayed right back to you.

Congrats, you have used Pion WebRTC! Now start building something cool
