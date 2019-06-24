// +build !js

package webrtc

import (
	"testing"
)

var sdpNoFingerprintInFirstMedia = `v=0
o=- 143087887 1561022767 IN IP4 192.168.84.254
s=VideoRoom 404986692241682
t=0 0
a=group:BUNDLE audio
a=msid-semantic: WMS 2867270241552712
m=video 0 UDP/TLS/RTP/SAVPF 0
c=IN IP4 192.168.84.254
a=inactive
m=audio 9 UDP/TLS/RTP/SAVPF 111
c=IN IP4 192.168.84.254
a=recvonly
a=mid:audio
a=rtcp-mux
a=ice-ufrag:AS/w
a=ice-pwd:9NOgoAOMALYu/LOpA1iqg/
a=ice-options:trickle
a=fingerprint:sha-256 D2:B9:31:8F:DF:24:D8:0E:ED:D2:EF:25:9E:AF:6F:B8:34:AE:53:9C:E6:F3:8F:F2:64:15:FA:E8:7F:53:2D:38
a=setup:active
a=rtpmap:111 opus/48000/2
a=candidate:1 1 udp 2013266431 192.168.84.254 46492 typ host
a=end-of-candidates
`

func TestNoFingerprintInFirstMediaIfSetRemoteDescription(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	desc := SessionDescription{
		Type: SDPTypeOffer,
		SDP:  sdpNoFingerprintInFirstMedia,
	}

	err = pc.SetRemoteDescription(desc)
	if err != nil {
		t.Error(err.Error())
	}
}
