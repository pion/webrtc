# ortc
ortc demonstrates Pion WebRTC's [ORTC](https://ortc.org/) capabilities. Instead of using the Session Description Protocol
to configure and communicate ORTC provides APIs. Users then can implement signaling with whatever protocol they wish.
ORTC can then be used to implement WebRTC. A ORTC implementation can parse/emit Session Description and act as a WebRTC
implementation.

In this example we have defined a simple JSON based signaling protocol.

## Instructions
### Download ortc
```
go install github.com/pion/webrtc/v4/examples/ortc@latest
```

### Run first client as offerer
`ortc -offer` this will emit a base64 message. Copy this message to your clipboard.

## Run the second client as answerer
Run the second client. This should be launched with the message you copied in the previous step as stdin.

`echo $BASE64_MESSAGE_YOU_COPIED | ortc`

This will emit another base64 message. Copy this new message.

## Send base64 message to first client via CURL

* Run `curl localhost:8080 -d "BASE64_MESSAGE_YOU_COPIED"`. `BASE64_MESSAGE_YOU_COPIED` is the value you copied in the last step.

### Enjoy
If everything worked you will see `Data channel 'Foo'-'' open.` in each terminal.

Each client will send random messages every 5 seconds that will appear in the terminal

Congrats, you have used Pion WebRTC! Now start building something cool
