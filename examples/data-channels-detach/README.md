# data-channels-detach
data-channels-detach is an example that shows how you can detach a data channel. This allows direct access the the underlying [pion/datachannel](https://github.com/pion/datachannel). This allows you to interact with the data channel using a more idiomatic API based on the `io.ReadWriteCloser` interface.

The example mirrors the data-channels example.

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Fdata-channels-detach%2Fdrawio)

## Install
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/data-channels-detach
```

## Usage
The example can be used in the same way as the data-channel example or can be paired with the data-channels-detach-create example. In the latter case; run both example and exchange the offer/answer text by copy-pasting them on the other terminal.
