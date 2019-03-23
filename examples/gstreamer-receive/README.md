# gstreamer-receive
gstreamer-receive is a simple application that shows how to receive media using pion-WebRTC and play live using GStreamer.

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
[jsfiddle.net](https://jsfiddle.net/pdm7bqfr/) you should see your Webcam, two text-areas and a 'Start Session' button

### Run gstreamer-receive with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser, copy that and:
#### Linux/macOS
Run `echo $BROWSER_SDP | gstreamer-receive`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `gstreamer-receive < my_file`

### Input gstreamer-receive's SessionDescription into your browser
Copy the text that `gstreamer-receive` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your media!
Your video and/or audio should popup automatically, and will continue playing until you close the application.

Congrats, you have used pion-WebRTC! Now start building something cool
