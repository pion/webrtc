package webrtc

import (
	"strconv"

	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pions/webrtc/pkg/rtp/codecs"
	"github.com/pkg/errors"
)

// RegisterCodec is used to register a codec with the DefaultMediaEngine
func RegisterCodec(codec *RTCRtpCodec) {
	DefaultMediaEngine.RegisterCodec(codec)
}

// TODO: Phase out DefaultPayloadTypes in favor or dynamic assignment in 96-127 range

// PayloadTypes for the default codecs
const (
	DefaultPayloadTypeG722 = 9
	DefaultPayloadTypeOpus = 111
	DefaultPayloadTypeVP8  = 96
	DefaultPayloadTypeVP9  = 98
	DefaultPayloadTypeH264 = 100
)

// RegisterDefaultCodecs is a helper that registers the default codecs supported by pions-webrtc
func RegisterDefaultCodecs() {
	RegisterCodec(NewRTCRtpG722Codec(DefaultPayloadTypeG722, 8000, 1))
	RegisterCodec(NewRTCRtpOpusCodec(DefaultPayloadTypeOpus, 48000, 2))
	RegisterCodec(NewRTCRtpVP8Codec(DefaultPayloadTypeVP8, 90000))
	RegisterCodec(NewRTCRtpH264Codec(DefaultPayloadTypeH264, 90000))
	RegisterCodec(NewRTCRtpVP9Codec(DefaultPayloadTypeVP9, 90000))
}

// DefaultMediaEngine is the default MediaEngine used by RTCPeerConnections
var DefaultMediaEngine = NewMediaEngine()

// NewMediaEngine creates a new MediaEngine
func NewMediaEngine() *MediaEngine {
	return &MediaEngine{}
}

// MediaEngine defines the codecs supported by a RTCPeerConnection
type MediaEngine struct {
	codecs []*RTCRtpCodec
}

// RegisterCodec registers a codec to a media engine
func (m *MediaEngine) RegisterCodec(codec *RTCRtpCodec) uint8 {
	// TODO: generate PayloadType if not set
	m.codecs = append(m.codecs, codec)
	return codec.PayloadType
}

func (m *MediaEngine) getCodec(payloadType uint8) (*RTCRtpCodec, error) {
	for _, codec := range m.codecs {
		if codec.PayloadType == payloadType {
			return codec, nil
		}
	}
	return nil, errors.New("Codec not found")
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
func NewRTCRtpG722Codec(payloadType uint8, clockrate uint32, channels uint16) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeAudio,
		G722,
		clockrate,
		channels,
		"",
		payloadType,
		&codecs.G722Payloader{})
	return c
}

// NewRTCRtpOpusCodec is a helper to create an Opus codec
func NewRTCRtpOpusCodec(payloadType uint8, clockrate uint32, channels uint16) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeAudio,
		Opus,
		clockrate,
		channels,
		"minptime=10;useinbandfec=1",
		payloadType,
		&codecs.OpusPayloader{})
	return c
}

// NewRTCRtpVP8Codec is a helper to create an VP8 codec
func NewRTCRtpVP8Codec(payloadType uint8, clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeVideo,
		VP8,
		clockrate,
		0,
		"",
		payloadType,
		&codecs.VP8Payloader{})
	return c
}

// NewRTCRtpVP9Codec is a helper to create an VP9 codec
func NewRTCRtpVP9Codec(payloadType uint8, clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeVideo,
		VP9,
		clockrate,
		0,
		"",
		payloadType,
		nil) // TODO
	return c
}

// NewRTCRtpH264Codec is a helper to create an H264 codec
func NewRTCRtpH264Codec(payloadType uint8, clockrate uint32) *RTCRtpCodec {
	c := NewRTCRtpCodec(RTCRtpCodecTypeVideo,
		H264,
		clockrate,
		0,
		"level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		payloadType,
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
	payloadType uint8,
	payloader rtp.Payloader,
) *RTCRtpCodec {
	return &RTCRtpCodec{
		RTCRtpCodecCapability: RTCRtpCodecCapability{
			MimeType:    codecType.String() + "/" + name,
			ClockRate:   clockrate,
			Channels:    channels,
			SdpFmtpLine: fmtp,
		},
		PayloadType: payloadType,
		Payloader:   payloader,
		Type:        codecType,
		Name:        name,
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
