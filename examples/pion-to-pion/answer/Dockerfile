FROM golang:1.17

ENV GO111MODULE=on
RUN go get -u github.com/pion/webrtc/v3/examples/pion-to-pion/answer

CMD ["answer"]

EXPOSE 50000
