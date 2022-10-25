# rtcp-processing
rtcp-processing demonstrates the Public API for processing RTCP packets in Pion WebRTC.

This example is only processing messages for a RTPReceiver. A RTPReceiver is used for accepting
media from a remote peer.  These APIs also exist on the RTPSender when sending media to a remote peer.

RTCP is used for statistics and control information for media in WebRTC. Using these messages
you can get information about the quality of the media, round trip time and packet loss. You can
also craft messages to influence the media quality.

## Instructions
### Download rtcp-processing
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/rtcp-processing
```

### Open rtcp-processing example page
[jsfiddle.net](https://jsfiddle.net/zurq6j7x/) you should see two text-areas, 'Start Session' button and 'Copy browser SessionDescription to clipboard'

### Run rtcp-processing with your browsers Session Description as stdin
In the jsfiddle press 'Copy browser Session Description to clipboard' or copy the base64 string manually.

Now use this value you just copied as the input to `rtcp-processing`

#### Linux/macOS
Run `echo $BROWSER_SDP | rtcp-processing`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `rtcp-processing < my_file`

### Input rtcp-processing's Session Description into your browser
Copy the text that `rtcp-processing` just emitted and copy into the second text area in the jsfiddle

### Hit 'Start Session' in jsfiddle
You will see console messages for each inbound RTCP message from the remote peer.

Congrats, you have used Pion WebRTC! Now start building something cool
