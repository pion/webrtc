<h1 align="center">
  Examples
</h1>

We've build an extensive collection of examples covering common use-cases. You can modify and extend these examples to quickly get started.

For more full featured examples that use 3rd party libraries see our **[example-webrtc-applications](https://github.com/pion/example-webrtc-applications)** repo.

### Overview
#### Media API
* [Reflect](reflect): The reflect example demonstrates how to have Pion send back to the user exactly what it receives using the same PeerConnection.
* [Play from Disk](play-from-disk): The play-from-disk example demonstrates how to send video to your browser from a file saved to disk.
* [Play from Disk Renegotation](play-from-disk-renegotation): The play-from-disk-renegotation example is an extension of the play-from-disk example, but demonstrates how you can add/remove video tracks from an already negotiated PeerConnection.
* [Insertable Streams](insertable-streams): The insertable-streams example demonstrates how Pion can be used to send E2E encrypted video and decrypt via insertable streams in the browser.
* [Save to Disk](save-to-disk): The save-to-disk example shows how to record your webcam and save the footage to disk on the server side.
* [Broadcast](broadcast): The broadcast example demonstrates how to broadcast a video to multiple peers. A broadcaster uploads the video once and the server forwards it to all other peers.
* [RTP Forwarder](rtp-forwarder): The rtp-forwarder example demonstrates how to forward your audio/video streams using RTP.
* [RTP to WebRTC](rtp-to-webrtc): The rtp-to-webrtc example demonstrates how to take RTP packets sent to a Pion process into your browser.

#### Data Channel API
* [Data Channels](data-channels): The data-channels example shows how you can send/recv DataChannel messages from a web browser.
* [Data Channels Create](data-channels-create): Example data-channels-create shows how you can send/recv DataChannel messages from a web browser. The difference with the data-channels example is that the data channel is initialized from the server side in this example.
* [Data Channels Close](data-channels-close): Example data-channels-close is a variant of data-channels that allow playing with the life cycle of data channels.
* [Data Channels Detach](data-channels-detach): The data-channels-detach example shows how you can send/recv DataChannel messages using the underlying DataChannel implementation directly. This provides a more idiomatic way of interacting with Data Channels.
* [Data Channels Detach Create](data-channels-detach-create): Example data-channels-detach-create shows how you can send/recv DataChannel messages using the underlying DataChannel implementation directly. This provides a more idiomatic way of interacting with Data Channels. The difference with the data-channels-detach example is that the data channel is initialized in this example.
* [ORTC](ortc): Example ortc shows how you an use the ORTC API for DataChannel communication.
* [ORTC QUIC](ortc-quic): Example ortc-quic shows how you an use the ORTC API for QUIC DataChannel communication.
* [Pion to Pion](pion-to-pion): Example pion-to-pion is an example of two pion instances communicating directly! It therefore has no corresponding web page.

#### Miscellaneous
* [Custom Logger](custom-logger) The custom-logger demonstrates how the user can override the logging and process messages instead of printing to stdout. It has no corresponding web page.

### Usage
We've made it easy to run the browser based examples on your local machine.

1. Build and run the example server:
    ``` sh
    GO111MODULE=on go get github.com/pion/webrtc/v2
    git clone https://github.com/pion/webrtc.git $GOPATH/src/github.com/pion/webrtc
    cd $GOPATH/src/github.com/pion/webrtc/examples
    go run examples.go
    ```

2. Browse to [localhost](http://localhost) to browse through the examples. Note that you can change the port of the server using the ``--address`` flag:
    ``` sh
    go run examples.go --address localhost:8080
    go run examples.go --address :8080            # listen on all available interfaces
    ```

### WebAssembly
Some of our examples have support for WebAssembly. The same examples server documented above can be used to run the WebAssembly examples. However, you have to compile them first. This is done as follows:

1. If the example supports WebAssembly it will contain a `main.go` file under the `jsfiddle` folder.
2. Build this `main.go` file as follows:
    ```
    GOOS=js GOARCH=wasm go build -o demo.wasm
    ```
3. Start the example server. Refer to the [usage](#usage) section for how you can build the example server.
4. Browse to [localhost](http://localhost). The page should now give you the option to run the example using the WebAssembly binary.
