# custom-logger

custom-logger is an example of how the Pion API provides an customizable
logging API. By default all Pion projects log to stdout, but we also allow
users to override this and process messages however they want.

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc-pion%2Fmaster%2Fexamples%2Fcustom-logger%2Fdrawio)

## Instructions

### Download custom-logger

```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/custom-logger
```

### Run custom-logger

`custom-logger`

You should see messages from our customLogger, as two PeerConnections start a session
