# reflect
reflect demonstrates how with one PeerConnection you can send video to Pion and have the packets sent back. This example could be easily extended to do server side processing.

## (architecture)[https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Freflect%2Fdrawio]

## Instructions
### Download reflect
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/reflect
```

### Open reflect example page
[jsfiddle.net](https://jsfiddle.net/9jgukzt1/) you should see two text-areas and a 'Start Session' button.

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
