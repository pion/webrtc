# gstreamer-receive
gstreamer-receive is a simple application that shows how to record your webcam using pion-WebRTC and play live using GStreamer.

## Instructions
### Install GStreamer
This example requires you have GStreamer installed, these are the supported platforms
#### Debian/Ubuntu
`sudo apt-get install libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev gstreamer1.0-plugins-good`
#### Windows MinGW64/MSYS2
`pacman -S mingw-w64-x86_64-gstreamer mingw-w64-x86_64-gst-libav mingw-w64-x86_64-gst-plugins-good mingw-w64-x86_64-gst-plugins-bad mingw-w64-x86_64-gst-plugins-ugly`
### Download gstreamer-receive
```
go get github.com/pions/webrtc/examples/gstreamer-receive
```

### Open gstreamer-receive example page
[jsfiddle.net](https://jsfiddle.net/tr2uq31e/1/) you should see your Webcam, two text-areas and a 'Start Session' button

### Run gstreamer-receive, input browser's SessionDescription
In the jsfiddle the top textarea is your browsers, copy that and paste into `gstreamer-receive` and press enter

### Input gstreamer-receive's SessionDescription into your browser
Copy the text that `gstreamer-receive` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
Your video should popup automatically, and will continue playing until you close the application.

Congrats, you have used pion-WebRTC! Now start building something cool
