module github.com/pion/webrtc/v3

go 1.17

require (
	github.com/pion/datachannel v1.5.5
	github.com/pion/dtls/v2 v2.2.7
	github.com/pion/ice/v2 v2.3.24
	github.com/pion/interceptor v0.1.25
	github.com/pion/logging v0.2.2
	github.com/pion/randutil v0.1.0
	github.com/pion/rtcp v1.2.12
	github.com/pion/rtp v1.8.5
	github.com/pion/sctp v1.8.16
	github.com/pion/sdp/v3 v3.0.9
	github.com/pion/srtp/v2 v2.0.18
	github.com/pion/stun v0.6.1
	github.com/pion/transport/v2 v2.2.4
	github.com/sclevine/agouti v3.0.0+incompatible
	github.com/stretchr/testify v1.9.0
	golang.org/x/net v0.22.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.17.0 // indirect
	github.com/pion/mdns v0.0.12 // indirect
	github.com/pion/turn/v2 v2.1.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// SCTP ZeroChecksum implementation has a interoperability bug
// 3.2.28 can only work against itself, not other versions of webrtc
retract v3.2.28
