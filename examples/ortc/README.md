# ortc
ortc demonstrates Pion WebRTC's [ORTC](https://ortc.org/) capabilities. Instead of using the Session Description Protocol
to configure and communicate ORTC provides APIs. Users then can implement signaling with whatever protocol they wish.
ORTC can then be used to implement WebRTC. A ORTC implementation can parse/emit Session Description and act as a WebRTC
implementation.

In this example we have defined a simple JSON based signaling protocol.

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc%2Fmaster%2Fexamples%2Fortc%2Fdrawio)

## Instructions
### Download ortc
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/ortc
```

### Run first client as offerer
`ortc -offer` this will emit a base64 message. Copy this message to your clipboard.

## Run the second client as answerer
Run the second client. This should be launched with the message you copied in the previous step as stdin.

`echo BASE64_MESSAGE_YOU_COPIED | ortc`

### Enjoy
If everything worked you will see `Data channel 'Foo'-'' open.` in each terminal.

Each client will send random messages every 5 seconds that will appear in the terminal

Congrats, you have used Pion WebRTC! Now start building something cool
