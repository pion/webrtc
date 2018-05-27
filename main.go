package main

import (
	"fmt"

	"github.com/pions/webrtc/internal/sdp"
)

var offer string = `v=5
o=- 5170208399471905959 3 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE audio video
a=msid-semantic: WMS GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg
m=audio 9 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:62Xk
a=ice-pwd:wmdmbYe4GoL24xoECsY5dUDW
a=ice-options:trickle
a=fingerprint:sha-256 86:3E:1E:81:12:B2:3F:1B:8F:44:D9:6D:8C:A7:EA:93:AF:A3:EE:12:4A:51:3F:BE:45:4D:7F:58:F2:91:10:A3
a=setup:actpass
a=mid:audio
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=sendrecv
a=rtcp-mux
a=rtpmap:111 opus/48000/2
a=rtcp-fb:111 transport-cc
a=fmtp:111 minptime=10;useinbandfec=1
a=rtpmap:103 ISAC/16000
a=rtpmap:104 ISAC/32000
a=rtpmap:9 G722/8000
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:106 CN/32000
a=rtpmap:105 CN/16000
a=rtpmap:13 CN/8000
a=rtpmap:110 telephone-event/48000
a=rtpmap:112 telephone-event/32000
a=rtpmap:113 telephone-event/16000
a=rtpmap:126 telephone-event/8000
a=ssrc:2887999179 cname:xVOr2VlleqfWPmYI
a=ssrc:2887999179 msid:GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg d11be3f4-352b-4873-bef6-3140ddddad32
a=ssrc:2887999179 mslabel:GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg
a=ssrc:2887999179 label:d11be3f4-352b-4873-bef6-3140ddddad32
m=video 9 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 102
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:62Xk
a=ice-pwd:wmdmbYe4GoL24xoECsY5dUDW
a=ice-options:trickle
a=fingerprint:sha-256 86:3E:1E:81:12:B2:3F:1B:8F:44:D9:6D:8C:A7:EA:93:AF:A3:EE:12:4A:51:3F:BE:45:4D:7F:58:F2:91:10:A3
a=setup:actpass
a=mid:video
a=extmap:2 urn:ietf:params:rtp-hdrext:toffset
a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=extmap:4 urn:3gpp:video-orientation
a=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay
a=extmap:7 http://www.webrtc.org/experiments/rtp-hdrext/video-content-type
a=extmap:8 http://www.webrtc.org/experiments/rtp-hdrext/video-timing
a=sendrecv
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
a=rtpmap:98 VP9/90000
a=rtcp-fb:98 goog-remb
a=rtcp-fb:98 transport-cc
a=rtcp-fb:98 ccm fir
a=rtcp-fb:98 nack
a=rtcp-fb:98 nack pli
a=rtpmap:99 rtx/90000
a=fmtp:99 apt=98
a=rtpmap:100 red/90000
a=rtpmap:101 rtx/90000
a=fmtp:101 apt=100
a=rtpmap:102 ulpfec/90000
a=ssrc-group:FID 2760193303 4032463893
a=ssrc:2760193303 cname:xVOr2VlleqfWPmYI
a=ssrc:2760193303 msid:GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg d4fc8e95-10b3-4772-9a17-ec7e3c5d3c10
a=ssrc:2760193303 mslabel:GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg
a=ssrc:2760193303 label:d4fc8e95-10b3-4772-9a17-ec7e3c5d3c10
a=ssrc:4032463893 cname:xVOr2VlleqfWPmYI
a=ssrc:4032463893 msid:GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg d4fc8e95-10b3-4772-9a17-ec7e3c5d3c10
a=ssrc:4032463893 mslabel:GCn7YH8sORLpim711LJvDoE5IX1awTl3EcOg
a=ssrc:4032463893 label:d4fc8e95-10b3-4772-9a17-ec7e3c5d3c10
`

var answer string = `v=0
o=- 7350794434983194469 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE audio video
a=msid-semantic: WMS
m=audio 9 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:2ukV
a=ice-pwd:hAhdXfH8H8+IqWTdIj7q+XBa
a=ice-options:trickle
a=fingerprint:sha-256 26:12:82:57:36:FC:E3:3A:51:2E:A0:A8:33:BA:CC:A1:CD:9E:0B:FD:B5:CE:00:8C:23:F8:3B:A8:C4:B0:17:87
a=setup:active
a=mid:audio
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=recvonly
a=rtcp-mux
a=rtpmap:111 opus/48000/2
a=rtcp-fb:111 transport-cc
a=fmtp:111 minptime=10;useinbandfec=1
a=rtpmap:103 ISAC/16000
a=rtpmap:104 ISAC/32000
a=rtpmap:9 G722/8000
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:106 CN/32000
a=rtpmap:105 CN/16000
a=rtpmap:13 CN/8000
a=rtpmap:110 telephone-event/48000
a=rtpmap:112 telephone-event/32000
a=rtpmap:113 telephone-event/16000
a=rtpmap:126 telephone-event/8000
m=video 9 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 102
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:2ukV
a=ice-pwd:hAhdXfH8H8+IqWTdIj7q+XBa
a=ice-options:trickle
a=fingerprint:sha-256 26:12:82:57:36:FC:E3:3A:51:2E:A0:A8:33:BA:CC:A1:CD:9E:0B:FD:B5:CE:00:8C:23:F8:3B:A8:C4:B0:17:87
a=setup:active
a=mid:video
a=extmap:2 urn:ietf:params:rtp-hdrext:toffset
a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=extmap:4 urn:3gpp:video-orientation
a=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay
a=extmap:7 http://www.webrtc.org/experiments/rtp-hdrext/video-content-type
a=extmap:8 http://www.webrtc.org/experiments/rtp-hdrext/video-timing
a=recvonly
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
a=rtpmap:98 VP9/90000
a=rtcp-fb:98 goog-remb
a=rtcp-fb:98 transport-cc
a=rtcp-fb:98 ccm fir
a=rtcp-fb:98 nack
a=rtcp-fb:98 nack pli
a=rtpmap:99 rtx/90000
a=fmtp:99 apt=98
a=rtpmap:100 red/90000
a=rtpmap:101 rtx/90000
a=fmtp:101 apt=100
a=rtpmap:102 ulpfec/90000
`

func main() {
	s := &sdp.SessionDescription{}
	fmt.Println(s.Marshal(offer))
	fmt.Printf("%+v \n", s)

	fmt.Println(s.Marshal(answer))
}
