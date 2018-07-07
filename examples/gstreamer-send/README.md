# gstreamer-send
gstreamer-send is a simple application that shows how to send video to your browser using pion-WebRTC and GStreamer.

## Instructions
### Install GStreamer
This example requires you have GStreamer installed, these are the supported platforms
#### Debian/Ubuntu
`sudo apt-get install libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev gstreamer1.0-plugins-good`
#### Windows MinGW64/MSYS2
`pacman -S mingw-w64-x86_64-gstreamer mingw-w64-x86_64-gst-libav mingw-w64-x86_64-gst-plugins-good mingw-w64-x86_64-gst-plugins-bad mingw-w64-x86_64-gst-plugins-ugly`
### Download gstreamer-send
```
go get github.com/pions/webrtc/examples/gstreamer-send
```

### Open gstreamer-send example page
[jsfiddle.net](https://jsfiddle.net/gh/get/library/pure/pions/webrtc/tree/master/examples/gstreamer-send/jsfiddle) you should see two text-areas and a 'Start Session' button

### Run gstreamer-send with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | gstreamer-send`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `gstreamer-send < my_file`

### Input gstreamer-send's SessionDescription into your browser
Copy the text that `gstreamer-send` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes, and will continue playing until you close the application.

Congrats, you have used pion-WebRTC! Now start building something cool
