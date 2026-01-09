# repacketize

repacketize demonstrates how many video codecs can be received, depacketized and packetized by Pion over RTP.

## Instructions

### Download and run repacketize

```
go install github.com/pion/webrtc/v4/examples/repacketize@latest
```

### Open repacketize local page

[localhost:8080](http://localhost:8080/) you should see a dropdown selector and a "Start" button.

### Select one of the video codecs

Availability of codecs depends on the browser you are accessing the page from; Safari and Google Chrome on Windows should support all of them.

### Hit 'Start', enjoy your video

Your browser should send video to Pion, and then it will be relayed right back to you.

Congrats, you have used Pion WebRTC! Now start building something cool.
