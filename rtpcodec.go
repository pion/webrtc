package webrtc

import (
	"strings"
)

// RTPCodecType determines the type of a codec
type RTPCodecType int

const (

	// RTPCodecTypeAudio indicates this is an audio codec
	RTPCodecTypeAudio RTPCodecType = iota + 1

	// RTPCodecTypeVideo indicates this is a video codec
	RTPCodecTypeVideo
)

func (t RTPCodecType) String() string {
	switch t {
	case RTPCodecTypeAudio:
		return "audio"
	case RTPCodecTypeVideo:
		return "video" //nolint: goconst
	default:
		return ErrUnknownType.Error()
	}
}

// NewRTPCodecType creates a RTPCodecType from a string
func NewRTPCodecType(r string) RTPCodecType {
	switch {
	case strings.EqualFold(r, RTPCodecTypeAudio.String()):
		return RTPCodecTypeAudio
	case strings.EqualFold(r, RTPCodecTypeVideo.String()):
		return RTPCodecTypeVideo
	default:
		return RTPCodecType(0)
	}
}

// RTPCodecCapability provides information about codec capabilities.
//
// https://w3c.github.io/webrtc-pc/#dictionary-rtcrtpcodeccapability-members
type RTPCodecCapability struct {
	MimeType     string
	ClockRate    uint32
	Channels     uint16
	SDPFmtpLine  string
	RTCPFeedback []RTCPFeedback
}

// RTPHeaderExtensionCapability is used to define a RFC5285 RTP header extension supported by the codec.
//
// https://w3c.github.io/webrtc-pc/#dom-rtcrtpcapabilities-headerextensions
type RTPHeaderExtensionCapability struct {
	URI string
}

// RTPHeaderExtensionParameters enables an application to determine whether a header extension is configured for
// use within an RTCRtpSender or RTCRtpReceiver.
//
// https://w3c.github.io/webrtc-pc/#rtcrtpheaderextensionparameters
type RTPHeaderExtensionParameters struct {
	URI string
	ID  uint8
}

// RTPCodecParameters is a sequence containing the media codecs that an RtpSender
// will choose from, as well as entries for RTX, RED and FEC mechanisms. This also
// includes the PayloadType that has been negotiated
//
// https://w3c.github.io/webrtc-pc/#rtcrtpcodecparameters
type RTPCodecParameters struct {
	RTPCodecCapability
	PayloadType PayloadType

	statsID string
}

// RTCRtpCapabilities is a list of supported codecs and header extensions
//
// https://w3c.github.io/webrtc-pc/#rtcrtpcapabilities
type RTCRtpCapabilities struct {
	HeaderExtensions []RTPHeaderExtensionCapability
	Codecs           []RTPCodecCapability
}

// Do a fuzzy find for a codec in the list of codecs
// Used for lookup up a codec in an existing list to find a match
func codecParametersFuzzySearch(needle RTPCodecParameters, haystack []RTPCodecParameters) (RTPCodecParameters, error) {
	// First attempt to match on MimeType + SDPFmtpLine
	for _, c := range haystack {
		if strings.EqualFold(c.RTPCodecCapability.MimeType, needle.RTPCodecCapability.MimeType) &&
			c.RTPCodecCapability.SDPFmtpLine == needle.RTPCodecCapability.SDPFmtpLine {
			return c, nil
		}
	}

	// Fallback to just MimeType
	for _, c := range haystack {
		if strings.EqualFold(c.RTPCodecCapability.MimeType, needle.RTPCodecCapability.MimeType) {
			return c, nil
		}
	}

	return RTPCodecParameters{}, ErrCodecNotFound
}
