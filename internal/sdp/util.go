package sdp

import (
	"math/rand"
	"strconv"
	"strings"
)

// BaseSessionDescription generates a default SDP response that is ice-lite, initiates the DTLS session and supports VP8, VP9 and Opus
func BaseSessionDescription(iceUsername, icePassword, fingerprint string, candidates []string) *SessionDescription {
	addMediaCandidates := func(m *MediaDescription) *MediaDescription {
		m.Attributes = append(m.Attributes, candidates...)
		m.Attributes = append(m.Attributes, "end-of-candidates")
		return m
	}

	audioMediaDescription := &MediaDescription{
		MediaName:      "audio 9 RTP/SAVPF 111",
		ConnectionData: "IN IP4 127.0.0.1",
		Attributes: []string{
			"setup:active",
			"mid:audio",
			"sendrecv",
			"ice-ufrag:" + iceUsername,
			"ice-pwd:" + icePassword,
			"ice-lite",
			"fingerprint:sha-256 " + fingerprint,
			"rtcp-mux",
			"rtcp-rsize",
			"rtpmap:111 opus/48000/2",
			"fmtp:111 minptime=10;useinbandfec=1",
		},
	}

	videoMediaDescription := &MediaDescription{
		MediaName:      "video 9 RTP/SAVPF 96 98",
		ConnectionData: "IN IP4 127.0.0.1",
		Attributes: []string{
			"setup:active",
			"mid:video",
			"sendrecv",
			"ice-ufrag:" + iceUsername,
			"ice-pwd:" + icePassword,
			"ice-lite",
			"fingerprint:sha-256 " + fingerprint,
			"rtcp-mux",
			"rtcp-rsize",
			"rtpmap:96 VP8/90000",
			"rtpmap:98 VP9/90000",
		},
	}

	sessionID := strconv.FormatUint(uint64(rand.Uint32())<<32+uint64(rand.Uint32()), 10)
	return &SessionDescription{
		ProtocolVersion: 0,
		Origin:          "pion-webrtc " + sessionID + " 2 IN IP4 0.0.0.0",
		SessionName:     "-",
		Timing:          []string{"0 0"},
		Attributes: []string{
			"group:BUNDLE audio video",
			"msid-semantic: WMS",
		},
		MediaDescriptions: []*MediaDescription{
			addMediaCandidates(audioMediaDescription),
			addMediaCandidates(videoMediaDescription),
		},
	}
}

// GetCodecForPayloadType scans the SessionDescription for the given payloadType and returns the codec
func GetCodecForPayloadType(payloadType uint8, sd *SessionDescription) (ok bool, codec string) {
	for _, m := range sd.MediaDescriptions {
		for _, a := range m.Attributes {
			if strings.Contains(a, "rtpmap:"+strconv.Itoa(int(payloadType))) {
				split := strings.Split(a, " ")
				if len(split) == 2 {
					split := strings.Split(split[1], "/")
					return true, split[0]
				}
			}
		}
	}
	return false, ""
}
