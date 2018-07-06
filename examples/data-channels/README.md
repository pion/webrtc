# data-channels
TODO

## Instructions
### Download data-channels
```
go get github.com/pions/webrtc/examples/data-channels
```

### Open data-channels example page
[jsfiddle.net](http://jsfiddle.net/5hqt08fe/9/)

### Run data-channels, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | data-channels`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `data-channels < my_file`

### Input data-channels's SessionDescription into your browser
Copy the text that `data-channels` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle
TODO

Congrats, you have used pion-WebRTC! Now start building something cool
