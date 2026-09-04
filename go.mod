module github.com/pion/webrtc/v4

go 1.24.0

replace (
	github.com/pion/dtls/v3 => github.com/pion/dtls/v3 v3.1.3-0.20260902001837-a2624993668b
	github.com/pion/ice/v4 => github.com/pion/ice/v4 v4.4.2-0.20260904005021-0e522183c3b2
	github.com/pion/stun/v4 => github.com/pion/stun/v4 v4.0.1-0.20260903164631-9ebdd2632757
)

require (
	github.com/pion/datachannel v1.6.2
	github.com/pion/dtls/v3 v3.1.8
	github.com/pion/ice/v4 v4.4.2-0.20260904005021-0e522183c3b2
	github.com/pion/interceptor v0.1.47
	github.com/pion/logging v0.2.4
	github.com/pion/randutil v0.1.0
	github.com/pion/rtcp v1.2.17
	github.com/pion/rtp v1.10.5
	github.com/pion/sctp v1.11.1
	github.com/pion/sdp/v3 v3.0.19
	github.com/pion/srtp/v3 v3.0.13
	github.com/pion/stun/v4 v4.0.1-0.20260903164631-9ebdd2632757
	github.com/pion/transport/v4 v4.1.0
	github.com/pion/turn/v5 v5.1.0
	github.com/sclevine/agouti v3.0.0+incompatible
	github.com/stretchr/testify v1.12.1
	golang.org/x/net v0.50.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.17.0 // indirect
	github.com/pion/mdns/v2 v2.2.0 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/time v0.14.0 // indirect
)
