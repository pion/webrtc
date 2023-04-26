# stats
stats demonstrates how to use the [webrtc-stats](https://www.w3.org/TR/webrtc-stats/) implementation provided by Pion WebRTC.

This API gives you access to the statistical information about a PeerConnection. This can help you understand what is happening
during a session and why.

## Instructions
### Download stats
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/stats
```

### Open stats example page
[jsfiddle.net](https://jsfiddle.net/s179hacu/) you should see your Webcam, two text-areas and two buttons: `Copy browser SDP to clipboard`, `Start Session`.

### Run stats, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser's Session Description. Press `Copy browser SDP to clipboard` or copy the base64 string manually.
We will use this value in the next step.

#### Linux/macOS
Run `echo $BROWSER_SDP | stats`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `stats < my_file`

### Input stats' SessionDescription into your browser
Copy the text that `stats` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle
The `stats` program will now print the InboundRTPStreamStats for each incoming stream. You will see the following in
your console. The exact fields will change as we add more values.

```
Stats for: video/VP8
InboundRTPStreamStats:
        PacketsReceived: 1255
        PacketsLost: 0
        Jitter: 588.9559641717999
        LastPacketReceivedTimestamp: 2023-04-26 13:16:16.63591134 -0400 EDT m=+18.317378921
        HeaderBytesReceived: 25100
        BytesReceived: 1361125
        FIRCount: 0
        PLICount: 0
        NACKCount: 0
```

Congrats, you have used Pion WebRTC! Now start building something cool
