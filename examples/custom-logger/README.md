# custom-logger
custom-logger is an example of how the Pion API provides an customizable
logging API. By default all Pion projects log to stdout, but we also allow
users to override this and process messages however they want.

## Instructions
### Download custom-logger
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/custom-logger
```

### Run custom-logger
`custom-logger`


You should see messages from our customLogger, as two PeerConnections start a session
