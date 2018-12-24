# data-channels
data-channels-detach is an example that shows how you can detach a data channel. This allows direct access the the underlying [pions/datachannel](https://github.com/pions/datachannel). This allows you to interact with the data channel using a more idiomatic API based on the `io.ReadWriteCloser` interface.

The example mirrors the data-channels example.

## Install
```
go get github.com/pions/webrtc/examples/data-channels-detach
```

## Usage
The example can be used in the same way as the data-channel example or can be paired with the data-channels-detach-create example. In the latter case; run both example and exchange the offer/answer text by copy-pasting them on the other terminal.