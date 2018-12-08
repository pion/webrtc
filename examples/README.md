<h1 align="center">
  Examples
</h1>

We've build an extensive collection of examples covering common use-cases. Modify and extend these examples to quickly get started.

* [gstreamer-receive](gstreamer-receive/README.md): Play video and audio from your Webcam live using GStreamer
* [gstreamer-send](gstreamer-send/README.md): Send video generated from GStreamer to your browser
* [save-to-disk](save-to-disk/README.md): Save video from your Webcam to disk
* [data-channels](data-channels/README.md): Use data channels to send text between Pion WebRTC and your browser
* [data-channels-create](data-channels/README.md): Similar to data channels but now Pion initiates the creation of the data channel.
* [sfu](sfu/README.md): Broadcast a video to many peers, while only requiring the broadcaster to upload once
* [pion-to-pion](pion-to-pion/README.md): An example of two Pion instances communicating directly.

All examples can be executed on your local machine.

### Install
``` sh
go get github.com/pions/webrtc
cd $GOPATH/src/github.com/pions/webrtc/examples
go run examples.go
```
Note: you can change the port of the server using the ``--address`` flag.

Finally, browse to [localhost](http://localhost) to browse through the examples.
