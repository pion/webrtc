# echo
echo demonstrates how with one PeerConnection you can send video to Pion and have the packets sent back. This example could be easily extended to do server side processing.

## Instructions
### Download echo
```
go get github.com/pion/webrtc/examples/echo
```

### Open echo example page
[jsfiddle.net](https://jsfiddle.net/3m0zute8/) you should see two text-areas and a 'Start Session' button.

### Run echo, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | echo`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `echo < my_file`

### Input echo's SessionDescription into your browser
Copy the text that `echo` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
Your browser should send video to Pion, and then it will be relayed right back to you.

Congrats, you have used pion-WebRTC! Now start building something cool
