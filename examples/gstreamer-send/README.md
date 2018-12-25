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
[jsfiddle.net](https://jsfiddle.net/Laf7ujeo/164/) you should see two text-areas and a 'Start Session' button

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

## Customizing your video or audio
`gstreamer-send` also accepts the command line arguments `-video-src` and `-audio-src` allowing you to provide custom inputs.

When prototyping with GStreamer it is highly recommended that you enable debug output, this is done by setting the `GST_DEBUG` enviroment variable.
You can read about that [here](https://gstreamer.freedesktop.org/data/doc/gstreamer/head/gstreamer/html/gst-running.html) a good default value is `GST_DEBUG=*:3`

You can also prototype a GStreamer pipeline by using `gst-launch-1.0` to see how things look before trying them with `gstreamer-send` for the examples below you
also may need additional setup to enable extra video codecs like H264. The output from GST_DEBUG should give you hints

These pipelines work on Linux, they may have issues on other platforms. We would love PRs for more example pipelines that people find helpful!

* a webcam, with computer generated audio.

  `echo $BROWSER_SDP | gstreamer-send -video-src "autovideosrc ! video/x-raw, width=320, height=240 ! videoconvert ! queue"`

* a pre-recorded video, sintel.mkv is available [here](https://durian.blender.org/download/)

  `echo $BROWSER_SDP | gstreamer-send -video-src "uridecodebin uri=file:///tmp/sintel.mkv ! videoscale ! video/x-raw, width=320, height=240 ! queue " -audio-src "uridecodebin uri=file:///tmp/sintel.mkv ! queue ! audioconvert"`
