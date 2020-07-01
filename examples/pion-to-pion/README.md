# pion-to-pion
pion-to-pion is an example of two pion instances communicating directly!

The SDP offer and answer are exchanged automatically over HTTP.
The `answer` side acts like a HTTP server and should therefore be ran first.

## Instructions
First run `answer`:
```sh
export GO111MODULE=on
go install github.com/pion/webrtc/v3/examples/pion-to-pion/answer
answer
```
Next, run `offer`:
```sh
go install github.com/pion/webrtc/v3/examples/pion-to-pion/offer
offer
```

You should see them connect and start to exchange messages.

## You can use Docker-compose to start this example:
```sh
docker-compose up -d
```

Now, you can see message exchanging, using `docker logs`.
