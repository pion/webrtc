FROM golang:1.17-alpine3.13

RUN apk add --no-cache \
  chromium \
  chromium-chromedriver

ENV CGO_ENABLED=0

WORKDIR /go/src/github.com/pion/webrtc/acceptance-tests/e2e

CMD ["go", "test", "-tags=e2e", "-v", "."]
