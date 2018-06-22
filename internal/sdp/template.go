package sdp

import (
	"math/rand"
	"strconv"
)

// VP8OnlyDescription generates a default SDP response that is ice-lite, initiates the DTLS session and only supports VP8
func VP8OnlyDescription(iceUsername, icePassword, fingerprint string, candidates []string) *SessionDescription {
	videoMediaDescription := &MediaDescription{
		MediaName:      "video 7 RTP/SAVPF 96 97",
		ConnectionData: "IN IP4 127.0.0.1",
		Attributes: []string{
			"setup:active",
			"mid:video",
			"recvonly",
			"ice-ufrag:" + iceUsername,
			"ice-pwd:" + icePassword,
			"ice-options:renomination",
			"rtcp-mux",
			"rtcp-rsize",
			"rtpmap:96 VP8/90000",
			"rtpmap:97 rtx/90000",
			"fmtp:97 apt=96",
			"rtcp-fb:96 goog-remb",
			"rtcp-fb:96 ccm fir",
			"rtcp-fb:96 nack",
			"rtcp-fb:96 nack pli",
		},
	}

	videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, candidates...)
	videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, "end-of-candidates")

	more := []string{
		"ssrc:2581832418 cname:pionvideo",
		"ssrc:2581832418 msid:pion pionv0",
		"ssrc:2581832418 mslabel:pion",
		"ssrc:2581832418 label:pionv0",
	}
	videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, more...)

	// Generate only UDP host candidates for ICE

	sessionID := strconv.FormatUint(uint64(rand.Uint32())<<32+uint64(rand.Uint32()), 10)
	return &SessionDescription{
		ProtocolVersion: 0,
		Origin:          "pion-webrtc " + sessionID + " 2 IN IP4 0.0.0.0",
		SessionName:     "-",
		Timing:          []string{"0 0"},
		Attributes: []string{
			"ice-lite",
			"fingerprint:sha-256 " + fingerprint,
			"msid-semantic: WMS pion",
			"group:BUNDLE video",
		},
		MediaDescriptions: []*MediaDescription{
			videoMediaDescription,
		},
	}
}
