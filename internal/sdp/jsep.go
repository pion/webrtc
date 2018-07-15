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
		ProtocolVersion: 0,
		Origin: fmt.Sprintf(
			"- %d %d IN IP4 0.0.0.0",
			newSessionID(),
			time.Now().Unix(),
		),
		SessionName: "-",
		Timing:      []string{"0 0"},
		Attributes: []string{
			//	"ice-options:trickle", // TODO: implement trickle ICE
			"fingerprint:sha-256 " + fingerprint,
		},
	}

	if identity {
		d.WithPropertyAttribute(AttrKeyIdentity)
	}

	return d
}

// WithPropertyAttribute adds a property attribute 'a=key' to the session description
func (d *SessionDescription) WithPropertyAttribute(key string) *SessionDescription {
	d.Attributes = append(d.Attributes, key)
	return d
}

// WithValueAttribute adds a value attribute 'a=key:value' to the session description
func (d *SessionDescription) WithValueAttribute(key, value string) *SessionDescription {
	d.Attributes = append(d.Attributes, fmt.Sprintf("%s:%s", key, value))
	return d
}

// WithMedia adds a media description to the session description
func (d *SessionDescription) WithMedia(md *MediaDescription) *SessionDescription {
	d.MediaDescriptions = append(d.MediaDescriptions, md)
	return d
}

// NewJSEPMediaDescription creates a new MediaDescription with
// some settings that are required by the JSEP spec.
func NewJSEPMediaDescription(typ string, codecPrefs []string) *MediaDescription {
	// TODO: handle codecPrefs
	d := &MediaDescription{
		MediaName:      fmt.Sprintf("%s 9 UDP/TLS/RTP/SAVPF", typ), // TODO: other transports?
		ConnectionData: "IN IP4 0.0.0.0",
		Attributes:     []string{},
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
		WithValueAttribute("ssrc", fmt.Sprintf("%d cname:%s", ssrc, cname)). // Deprecated but not pased out?
		WithValueAttribute("ssrc", fmt.Sprintf("%d msid:%s %s", ssrc, streamLabel, label)).
		WithValueAttribute("ssrc", fmt.Sprintf("%d mslabel:%s", ssrc, streamLabel)). // Deprecated but not pased out?
		WithValueAttribute("ssrc", fmt.Sprintf("%d label:%s", ssrc, label))          // Deprecated but not pased out?
}

// WithCandidate adds an ICE candidate to the media description
func (d *MediaDescription) WithCandidate(id int, transport string, basePriority uint16, ip string, port int, typ string) *MediaDescription {
	return d.
		WithValueAttribute("candidate",
			fmt.Sprintf("%scandidate %d %s %d %s %d typ %s", transport, id, transport, basePriority, ip, port, typ))
}
