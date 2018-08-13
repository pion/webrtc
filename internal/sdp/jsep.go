package sdp

import (
	"fmt"
	"time"
)

// Constants for SDP attributes used in JSEP
const (
	AttrKeyIdentity        = "identity"
	AttrKeyGroup           = "group"
	AttrKeySsrc            = "ssrc"
	AttrKeySsrcGroup       = "ssrc-group"
	AttrKeyMsidSemantic    = "msid-semantic"
	AttrKeyConnectionSetup = "setup"
	AttrKeyMID             = "mid"
	AttrKeyICELite         = "ice-lite"
	AttrKeyRtcpMux         = "rtcp-mux"
	AttrKeyRtcpRsize       = "rtcp-rsize"
)

// Constants for semantic tokens used in JSEP
const (
	SemanticTokenLipSynchronization     = "LS"
	SemanticTokenFlowIdentification     = "FID"
	SemanticTokenForwardErrorCorrection = "FEC"
	SemanticTokenWebRTCMediaStreams     = "WMS"
)

// API to match draft-ietf-rtcweb-jsep
// Move to webrtc or its own package?

// NewJSEPSessionDescription creates a new SessionDescription with
// some settings that are required by the JSEP spec.
func NewJSEPSessionDescription(fingerprint string, identity bool) *SessionDescription {
	d := &SessionDescription{
		ProtocolVersion:  0,
		Origin:           Origin{"-", newSessionID(), uint64(time.Now().Unix()), "IN", "IP4", "0.0.0.0"},
		SessionName:      "-",
		TimeDescriptions: []TimeDescription{{Timing{0, 0}, nil}},
		Attributes: []Attribute{
			// 	"Attribute(ice-options:trickle)", // TODO: implement trickle ICE
			Attribute("fingerprint:sha-256 " + fingerprint),
		},
	}

	if identity {
		d.WithPropertyAttribute(AttrKeyIdentity)
	}

	return d
}

// WithPropertyAttribute adds a property attribute 'a=key' to the session description
func (s *SessionDescription) WithPropertyAttribute(key string) *SessionDescription {
	s.Attributes = append(s.Attributes, Attribute(key))
	return s
}

// WithValueAttribute adds a value attribute 'a=key:value' to the session description
func (s *SessionDescription) WithValueAttribute(key, value string) *SessionDescription {
	s.Attributes = append(s.Attributes, Attribute(fmt.Sprintf("%s:%s", key, value)))
	return s
}

// WithMedia adds a media description to the session description
func (s *SessionDescription) WithMedia(md *MediaDescription) *SessionDescription {
	s.MediaDescriptions = append(s.MediaDescriptions, *md)
	return s
}

// NewJSEPMediaDescription creates a new MediaName with
// some settings that are required by the JSEP spec.
func NewJSEPMediaDescription(codecType string, codecPrefs []string) *MediaDescription {
	// TODO: handle codecPrefs
	d := &MediaDescription{
		MediaName: MediaName{
			Media: codecType,
			Proto: "UDP/TLS/RTP/SAVPF",
		},
		ConnectionInformation: "IN IP4 0.0.0.0",
	}
	return d
}

// WithPropertyAttribute adds a property attribute 'a=key' to the media description
func (d *MediaDescription) WithPropertyAttribute(key string) *MediaDescription {
	d.Attributes = append(d.Attributes, key)
	return d
}

// WithValueAttribute adds a value attribute 'a=key:value' to the media description
func (d *MediaDescription) WithValueAttribute(key, value string) *MediaDescription {
	d.Attributes = append(d.Attributes, fmt.Sprintf("%s:%s", key, value))
	return d
}

// WithICECredentials adds ICE credentials to the media description
func (d *MediaDescription) WithICECredentials(username, password string) *MediaDescription {
	return d.
		WithValueAttribute("ice-ufrag", username).
		WithValueAttribute("ice-pwd", password)
}

// WithCodec adds codec information to the media description
func (d *MediaDescription) WithCodec(payloadType uint8, name string, clockrate uint32, channels uint16, fmtp string) *MediaDescription {
	d.MediaName = fmt.Sprintf("%s %d", d.MediaName, payloadType)
	rtpmap := fmt.Sprintf("%d %s/%d", payloadType, name, clockrate)
	if channels > 0 {
		rtpmap = rtpmap + fmt.Sprintf("/%d", channels)
	}
	d.WithValueAttribute("rtpmap", rtpmap)
	if fmtp != "" {
		d.WithValueAttribute("fmtp", fmt.Sprintf("%d %s", payloadType, fmtp))
	}
	return d
}

// WithMediaSource adds media source information to the media description
func (d *MediaDescription) WithMediaSource(ssrc uint32, cname, streamLabel, label string) *MediaDescription {
	return d.
		WithValueAttribute("ssrc", fmt.Sprintf("%d cname:%s", ssrc, cname)). // Deprecated but not phased out?
		WithValueAttribute("ssrc", fmt.Sprintf("%d msid:%s %s", ssrc, streamLabel, label)).
		WithValueAttribute("ssrc", fmt.Sprintf("%d mslabel:%s", ssrc, streamLabel)). // Deprecated but not phased out?
		WithValueAttribute("ssrc", fmt.Sprintf("%d label:%s", ssrc, label)) // Deprecated but not phased out?
}

// WithCandidate adds an ICE candidate to the media description
func (d *MediaDescription) WithCandidate(value string) *MediaDescription {
	return d.WithValueAttribute("candidate", value)
}
