# SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
# SPDX-License-Identifier: MIT

FROM golang:1.24

RUN go install github.com/pion/webrtc/v4/examples/pion-to-pion/answer@latest

CMD ["answer"]

EXPOSE 50000
