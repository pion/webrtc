# data-channels-detach-create
data-channels-detach-create is an example that shows how you can detach a data channel. This allows direct access the the underlying [pion/datachannel](https://github.com/pion/datachannel). This allows you to interact with the data channel using a more idiomatic API based on the `io.ReadWriteCloser` interface.

The example mirrors the data-channels-create example.

## Install
```
go get github.com/pion/webrtc/v2/examples/data-channels-detach-create
```

## Usage
The example can be used in the same way as the data-channel example or can be paired with the data-channels-detach example. In the latter case; run both example and exchange the offer/answer text by copy-pasting them on the other terminal.
