package webrtc

import (
	"strconv"

	"github.com/pions/sdp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pions/webrtc/pkg/rtp/codecs"
	"github.com/pkg/errors"
)

// RegisterCodec is used to register a codec with the DefaultMediaEngine
func RegisterCodec(codec *RTCRtpCodec) {
	DefaultMediaEngine.RegisterCodec(codec)
}

// RegisterDefaultCodecs is a helper that registers the default codecs supported by pions-webrtc
func RegisterDefaultCodecs() {
	RegisterCodec(NewRTCRtpOpusCodec(48000, 2))
	RegisterCodec(NewRTCRtpG722Codec(8000))
	RegisterCodec(NewRTCRtpVP8Codec(90000))
	RegisterCodec(NewRTCRtpH264Codec(90000))
	RegisterCodec(NewRTCRtpVP9Codec(90000))
}

// DefaultMediaEngine is the default MediaEngine used by RTCPeerConnections
var DefaultMediaEngine = NewMediaEngine()

// NewMediaEngine creates a new MediaEngine
func NewMediaEngine() *MediaEngine {
	return &MediaEngine{nextPayloadType: 96}
}

// MediaEngine defines the codecs supported by a RTCPeerConnection
type MediaEngine struct {
	codecs []*RTCRtpCodec

	// store the next payload type used for dynamic assignment
	nextPayloadType uint8
}

// RegisterCodec registers a codec to a media engine
func (m *MediaEngine) RegisterCodec(codec *RTCRtpCodec) uint8 {

	var payloadType uint8

	// check for existing payload type
	for _, c := range m.codecs {
		if c.Name == codec.Name {
			payloadType = c.PayloadType
			break
		}
	}

	if payloadType > 0 {
		codec.PayloadType = payloadType
	} else if m.nextPayloadType >= 96 && m.nextPayloadType <= 127 {
		codec.PayloadType = m.nextPayloadType
		m.nextPayloadType++
	}

	m.codecs = append(m.codecs, codec)
	return codec.PayloadType
}

func (m *MediaEngine) getCodecSDP(sdpCodec sdp.Codec) (*RTCRtpCodec, error) {
	for _, codec := range m.codecs {
		if codec.Name == sdpCodec.Name &&
			codec.ClockRate == sdpCodec.ClockRate &&
			(sdpCodec.EncodingParameters == "" ||
				strconv.Itoa(int(codec.Channels)) == sdpCodec.EncodingParameters) &&
			codec.SdpFmtpLine == sdpCodec.Fmtp { // TODO: Protocol specific matching?
			return codec, nil
		}
	}
	return nil, errors.New("Codec not found")
}

func (m *MediaEngine) getCodecsByKind(kind RTCRtpCodecType) []*RTCRtpCodec {
	var codecs []*RTCRtpCodec
	for _, codec := range m.codecs {
		if codec.Type == kind {
			codecs = append(codecs, codec)
		}
	}
	return codecs
}

// Names for the default codecs supported by pions-webrtc
const (
	G722 = "G722"
	Opus = "opus"
	VP8  = "VP8"
	VP9  = "VP9"
	H264 = "H264"
)

// NewRTCRtpG722Codec is a helper to create a G722 codec
func NewRTCRtpG722Codec(clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeAudio,
		G722,
		clockrate,
		0,
		"",
		&codecs.G722Payloader{})
	return c
}

// NewRTCRtpOpusCodec is a helper to create an Opus codec
func NewRTCRtpOpusCodec(clockrate uint32, channels uint16) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeAudio,
		Opus,
		clockrate,
		channels,
		"minptime=10;useinbandfec=1",
		&codecs.OpusPayloader{})
	return c
}

// NewRTCRtpVP8Codec is a helper to create an VP8 codec
func NewRTCRtpVP8Codec(clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeVideo,
		VP8,
		clockrate,
		0,
		"",
		&codecs.VP8Payloader{})
	return c
}

// NewRTCRtpVP9Codec is a helper to create an VP9 codec
func NewRTCRtpVP9Codec(clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeVideo,
		VP9,
		clockrate,
		0,
		"",
		nil) // TODO
	return c
}

// NewRTCRtpH264Codec is a helper to create an H264 codec
func NewRTCRtpH264Codec(clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeVideo,
		H264,
		clockrate,
		0,
		"level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		&codecs.H264Payloader{})
	return c
}

// RTCRtpCodecType determines the type of a codec
type RTCRtpCodecType int

const (

	// RTCRtpCodecTypeAudio indicates this is an audio codec
	RTCRtpCodecTypeAudio RTCRtpCodecType = iota + 1

	// RTCRtpCodecTypeVideo indicates this is a video codec
	RTCRtpCodecTypeVideo
)

func (t RTCRtpCodecType) String() string {
	switch t {
	case RTCRtpCodecTypeAudio:
		return "audio"
	case RTCRtpCodecTypeVideo:
		return "video"
	default:
		return ErrUnknownType.Error()
	}
}

// RTCRtpCodec represents a codec supported by the PeerConnection
type RTCRtpCodec struct {
	RTCRtpCodecCapability
	Type        RTCRtpCodecType
	Name        string
	PayloadType uint8
	Payloader   rtp.Payloader
}

// NewRTCRtpCodec is used to define a new codec
func NewRTCRtpCodec(
	codecType RTCRtpCodecType,
	name string,
	clockrate uint32,
	channels uint16,
	fmtp string,
	payloader rtp.Payloader,
) *RTCRtpCodec {
	return &RTCRtpCodec{
		RTCRtpCodecCapability: RTCRtpCodecCapability{
			MimeType:    codecType.String() + "/" + name,
			ClockRate:   clockrate,
			Channels:    channels,
			SdpFmtpLine: fmtp,
		},
		Payloader: payloader,
		Type:      codecType,
		Name:      name,
	}
}

// RTCRtpCodecCapability provides information about codec capabilities.
type RTCRtpCodecCapability struct {
	MimeType    string
	ClockRate   uint32
	Channels    uint16
	SdpFmtpLine string
}

// RTCRtpHeaderExtensionCapability is used to define a RFC5285 RTP header extension supported by the codec.
type RTCRtpHeaderExtensionCapability struct {
	URI string
}

// RTCRtpCapabilities represents the capabilities of a transceiver
type RTCRtpCapabilities struct {
	Codecs           []RTCRtpCodecCapability
	HeaderExtensions []RTCRtpHeaderExtensionCapability
}
