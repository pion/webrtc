module github.com/pion/webrtc/v3

go 1.13

require (
	github.com/onsi/ginkgo v1.16.1 // indirect
	github.com/onsi/gomega v1.11.0 // indirect
	github.com/pion/datachannel v1.5.2
	github.com/pion/dtls/v2 v2.0.10
	github.com/pion/ice/v2 v2.1.14
	github.com/pion/interceptor v0.1.0
	github.com/pion/logging v0.2.2
	github.com/pion/randutil v0.1.0
	github.com/pion/rtcp v1.2.8
	github.com/pion/rtp v1.7.4
	github.com/pion/sctp v1.8.0
	github.com/pion/sdp/v3 v3.0.4
	github.com/pion/srtp/v2 v2.0.5
	github.com/pion/transport v0.12.3
	github.com/sclevine/agouti v3.0.0+incompatible
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20211020060615-d418f374d309
	golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359 // indirect
)

replace (
	github.com/pion/interceptor v0.1.0 => ../interceptor
	github.com/pion/transport v0.12.3 => ../transport
)
