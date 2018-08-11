package webrtc

import (
	"testing"
)

const minimalOffer = `v=0
o=- 7193157174393298413 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE video
m=video 43858 UDP/TLS/RTP/SAVPF 96
c=IN IP4 172.17.0.1
a=candidate:3885250869 1 udp 1 127.0.0.1 1 typ host
a=ice-ufrag:OgYk
a=ice-pwd:G0ka4ts7hRhMLNljuuXzqnOF
a=fingerprint:sha-256 D7:06:10:DE:69:66:B1:53:0E:02:33:45:63:F8:AF:78:B2:C7:CE:AF:8E:FD:E5:13:20:50:74:93:CD:B5:C8:69
a=setup:active
a=mid:video
a=sendrecv
a=rtpmap:96 VP8/90000
`

func TestSetRemoteDescription(t *testing.T) {
	testCases := []struct {
		desc RTCSessionDescription
	}{
		{RTCSessionDescription{Type: RTCSdpTypeOffer, Sdp: minimalOffer}},
	}

	for i, testCase := range testCases {
		peerConn, err := New(RTCConfiguration{})
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}
		err = peerConn.SetRemoteDescription(testCase.desc)
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}
	}
}
