FROM golang:1.19

ENV GO111MODULE=on
RUN go install github.com/pion/webrtc/v3/examples/pion-to-pion/offer@latest

CMD ["offer"]
