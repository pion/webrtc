FROM golang:1.20

ENV GO111MODULE=on
RUN go install github.com/pion/webrtc/v3/examples/pion-to-pion/answer@latest

CMD ["answer"]

EXPOSE 50000
