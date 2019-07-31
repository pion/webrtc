# pion-to-pion-trickle
pion-to-pion-trickle is an example of two pion instances communicating directly!
This example uses Trickle ICE, this allows communication to begin before gathering
has completed.

See `pion-to-pion` example of a non-Trickle version of this.

The SDP offer and answer are exchanged automatically over HTTP.
The `answer` side acts like a HTTP server and should therefore be ran first.

## Instructions
First run `answer`:
```sh
go install github.com/pion/webrtc/examples/pion-to-pion/answer
answer
```
Next, run `offer`:
```sh
go install github.com/pion/webrtc/examples/pion-to-pion/offer
offer
```

You should see them connect and start to exchange messages.
