# SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
# SPDX-License-Identifier: MIT

FROM golang:1.21-alpine

RUN apk add --no-cache \
  chromium \
  chromium-chromedriver

ENV CGO_ENABLED=0

COPY . /go/src/github.com/pion/webrtc
WORKDIR /go/src/github.com/pion/webrtc/e2e

CMD ["go", "test", "-tags=e2e", "-v", "."]
