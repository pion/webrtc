<h1 align="center">
  Examples
</h1>

We've built an extensive collection of examples covering common use-cases. You can modify and extend these examples to get started quickly.

For more full featured examples that use 3rd party libraries see our **[example-webrtc-applications](https://github.com/pion/example-webrtc-applications)** repo.

### Overview
#### Media API
* [Reflect](reflect): The reflect example demonstrates how to have Pion send back to the user exactly what it receives using the same PeerConnection.
* [Play from Disk](play-from-disk): The play-from-disk example demonstrates how to send video to your browser from a file saved to disk.
* [Play from Disk Renegotiation](play-from-disk-renegotiation): The play-from-disk-renegotiation example is an extension of the play-from-disk example, but demonstrates how you can add/remove video tracks from an already negotiated PeerConnection.
* [Insertable Streams](insertable-streams): The insertable-streams example demonstrates how Pion can be used to send E2E encrypted video and decrypt via insertable streams in the browser.
* [Save to Disk](save-to-disk): The save-to-disk example shows how to record your webcam and save the footage to disk on the server side.
* [Broadcast](broadcast): The broadcast example demonstrates how to broadcast a video to multiple peers. A broadcaster uploads the video once and the server forwards it to all other peers.
* [RTP Forwarder](rtp-forwarder): The rtp-forwarder example demonstrates how to forward your audio/video streams using RTP.
* [RTP to WebRTC](rtp-to-webrtc): The rtp-to-webrtc example demonstrates how to take RTP packets sent to a Pion process into your browser.
* [Simulcast](simulcast): The simulcast example demonstrates how to accept and demux 1 Track that contains 3 Simulcast streams. It then returns the media as 3 independent Tracks back to the sender.
* [Swap Tracks](swap-tracks): The swap-tracks example demonstrates deeper usage of the Pion Media API. The server accepts 3 media streams, and then dynamically routes them back as a single stream to the user.
* [RTCP Processing](rtcp-processing) The rtcp-processing example demonstrates Pion's RTCP APIs. This allow access to media statistics and control information.

#### Data Channel API
* [Data Channels](data-channels): The data-channels example shows how you can send/recv DataChannel messages from a web browser.
* [Data Channels Detach](data-channels-detach): The data-channels-detach example shows how you can send/recv DataChannel messages using the underlying DataChannel implementation directly. This provides a more idiomatic way of interacting with Data Channels.
* [Data Channels Flow Control](data-channels-flow-control): Example data-channels-flow-control shows how to use the DataChannel API efficiently. You can measure the amount the rate at which the remote peer is receiving data, and structure your application accordingly.
* [ORTC](ortc): Example ortc shows how you an use the ORTC API for DataChannel communication.
* [Pion to Pion](pion-to-pion): Example pion-to-pion is an example of two pion instances communicating directly! It therefore has no corresponding web page.

#### Miscellaneous
* [Custom Logger](custom-logger) The custom-logger demonstrates how the user can override the logging and process messages instead of printing to stdout. It has no corresponding web page.
* [ICE Restart](ice-restart) Example ice-restart demonstrates how a WebRTC connection can roam between networks. This example restarts ICE in a loop and prints the new addresses it uses each time.
* [ICE Single Port](ice-single-port) Example ice-single-port demonstrates how multiple WebRTC connections can be served from a single port. By default Pion listens on a new port for every PeerConnection. Pion can be configured to use a single port for multiple connections.
* [ICE TCP](ice-tcp) Example ice-tcp demonstrates how a WebRTC connection can be made over TCP instead of UDP. By default Pion only does UDP. Pion can be configured to use a TCP port, and this TCP port can be used for many connections.
* [Trickle ICE](trickle-ice) Example trickle-ice example demonstrates Pion WebRTC's Trickle ICE APIs. This is important to use since it allows ICE Gathering and Connecting to happen concurrently.
* [VNet](vnet) Example vnet demonstrates Pion's network virtualisation library. This example connects two PeerConnections over a virtual network and prints statistics about the data traveling over it.

### Usage
We've made it easy to run the browser based examples on your local machine.

1. Build and run the example server:
    ``` sh
    GO111MODULE=on go get github.com/pion/webrtc/v3
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
Pion WebRTC can be used when compiled to WebAssembly, also known as WASM. In
this case the library will act as a wrapper around the JavaScript WebRTC API.
This allows you to use WebRTC from Go in both server and browser side code with
little to no changes

Some of our examples have support for WebAssembly. The same examples server documented above can be used to run the WebAssembly examples. However, you have to compile them first. This is done as follows:

1. If the example supports WebAssembly it will contain a `main.go` file under the `jsfiddle` folder.
2. Build this `main.go` file as follows:
    ```
    GOOS=js GOARCH=wasm go build -o demo.wasm
    ```
3. Start the example server. Refer to the [usage](#usage) section for how you can build the example server.
4. Browse to [localhost](http://localhost). The page should now give you the option to run the example using the WebAssembly binary.
