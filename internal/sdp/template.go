package sdp

import (
	"math/rand"
	"strconv"
)

func VP8OnlyDescription(iceUsername, icePassword, fingerprint string, candidates []string) *SessionDescription {
	videoMediaDescription := &MediaDescription{
		MediaName:      "video 7 RTP/SAVPF 96 97",
		ConnectionData: "IN IP4 127.0.0.1",
		Attributes: []string{
			"rtpmap:96 VP8/90000",
			"rtpmap:97 rtx/90000",
			"fmtp:97 apt=96",
			"rtcp-fb:96 goog-remb",
			"rtcp-fb:96 ccm fir",
			"rtcp-fb:96 nack",
			"rtcp-fb:96 nack pli",
			"extmap:2 urn:ietf:params:rtp-hdrext:toffset",
			"extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
			"extmap:4 urn:3gpp:video-orientation",
			"setup:active",
			"mid:video",
			"recvonly",
			"ice-ufrag:" + iceUsername,
			"ice-pwd:" + icePassword,
			"ice-options:renomination",
			"rtcp-mux",
			"rtcp-rsize",
		},
	}

	for _, c := range candidates {
		videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, c)
	}
	videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, "end-of-candidates")

	// Generate only UDP host candidates for ICE

	sessionId := strconv.FormatUint(uint64(rand.Uint32())<<32+uint64(rand.Uint32()), 10)
	return &SessionDescription{
		ProtocolVersion: 0,
		Origin:          "pion-webrtc " + sessionId + " 2 IN IP4 0.0.0.0",
		SessionName:     "-",
		Timing:          []string{"0 0"},
		Attributes: []string{
			"ice-lite",
			"fingerprint:sha-256 " + fingerprint,
			"msid-semantic: WMS *",
			"group:BUNDLE video",
		},
		MediaDescriptions: []*MediaDescription{
			videoMediaDescription,
		},
	}

	return nil
}
