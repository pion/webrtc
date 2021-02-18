# simulcast
demonstrates of how to handle incoming track with multiple simulcast rtp streams and show all them back.

The browser will not send higher quality streams unless it has the available bandwidth. You can look at
the bandwidth estimation in `chrome://webrtc-internals`. It is under `VideoBwe` when `Read Stats From: Legacy non-Standard`
is selected.

## Instructions
### Download simulcast
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/simulcast
```

### Open simulcast example page
[jsfiddle.net](https://jsfiddle.net/rxk4bftc) you should see two text-areas and a 'Start Session' button.

### Run simulcast, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | simulcast`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `simulcast < my_file`

### Input simulcast's SessionDescription into your browser
Copy the text that `simulcast` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
Your browser should send a simulcast track to Pion, and then all 3 incoming streams will be relayed back.

Congrats, you have used Pion WebRTC! Now start building something cool
