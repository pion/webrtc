package webrtc

import (
	"strconv"

	"github.com/pions/rtp"
	"github.com/pions/rtp/codecs"
	"github.com/pions/sdp/v2"
)

// PayloadTypes for the default codecs
const (
	DefaultPayloadTypeG722 = 9
	DefaultPayloadTypeOpus = 111
	DefaultPayloadTypeVP8  = 96
	DefaultPayloadTypeVP9  = 98
	DefaultPayloadTypeH264 = 100
)

// A CodecList is a list of supported RTPCodecs for a PeerConnection.
//
// An empty CodecList is equivalent to DefaultCodecs, which contains RTP codecs
// commonly used with WebRTC.
type CodecList []*RTPCodec

// DefaultCodecs is a CodecList containing codecs commonly used with WebRTC
// and useful for most applications, including: Opus, G.722, VP8, H265, and VP9.
var DefaultCodecs CodecList = []*RTPCodec{
	NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000, 2),
	NewRTPG722Codec(DefaultPayloadTypeG722, 8000),
	NewRTPVP8Codec(DefaultPayloadTypeVP8, 90000),
	NewRTPH264Codec(DefaultPayloadTypeH264, 90000),
	NewRTPVP9Codec(DefaultPayloadTypeVP9, 90000),
}

func (c CodecList) getCodec(payloadType uint8) (*RTPCodec, error) {
	if c == nil {
		c = DefaultCodecs
	}

	for _, codec := range c {
		if codec.PayloadType == payloadType {
			return codec, nil
		}
	}
	return nil, ErrCodecNotFound
}

func (c CodecList) getCodecSDP(sdpCodec sdp.Codec) (*RTPCodec, error) {
	if c == nil {
		c = DefaultCodecs
	}

	for _, codec := range c {
		if codec.Name == sdpCodec.Name &&
			codec.ClockRate == sdpCodec.ClockRate &&
			(sdpCodec.EncodingParameters == "" ||
				strconv.Itoa(int(codec.Channels)) == sdpCodec.EncodingParameters) &&
			codec.SDPFmtpLine == sdpCodec.Fmtp { // TODO: Protocol specific matching?
			return codec, nil
		}
	}
	return nil, ErrCodecNotFound
}

func (c CodecList) getCodecsByKind(kind RTPCodecType) []*RTPCodec {
	if c == nil {
		c = DefaultCodecs
	}

	var codecs []*RTPCodec
	for _, codec := range c {
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

// NewRTPG722Codec is a helper to create a G722 codec
func NewRTPG722Codec(payloadType uint8, clockrate uint32) *RTPCodec {
	c := NewRTPCodec(RTPCodecTypeAudio,
		G722,
		clockrate,
		0,
		"",
		payloadType,
		&codecs.G722Payloader{})
	return c
}

// NewRTPOpusCodec is a helper to create an Opus codec
func NewRTPOpusCodec(payloadType uint8, clockrate uint32, channels uint16) *RTPCodec {
	c := NewRTPCodec(RTPCodecTypeAudio,
		Opus,
		clockrate,
		channels,
		"minptime=10;useinbandfec=1",
		payloadType,
		&codecs.OpusPayloader{})
	return c
}

// NewRTPVP8Codec is a helper to create an VP8 codec
func NewRTPVP8Codec(payloadType uint8, clockrate uint32) *RTPCodec {
	c := NewRTPCodec(RTPCodecTypeVideo,
		VP8,
		clockrate,
		0,
		"",
		payloadType,
		&codecs.VP8Payloader{})
	return c
}

// NewRTPVP9Codec is a helper to create an VP9 codec
func NewRTPVP9Codec(payloadType uint8, clockrate uint32) *RTPCodec {
	c := NewRTPCodec(RTPCodecTypeVideo,
		VP9,
		clockrate,
		0,
		"",
		payloadType,
		nil) // TODO
	return c
}

// NewRTPH264Codec is a helper to create an H264 codec
func NewRTPH264Codec(payloadType uint8, clockrate uint32) *RTPCodec {
	c := NewRTPCodec(RTPCodecTypeVideo,
		H264,
		clockrate,
		0,
		"level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		payloadType,
		&codecs.H264Payloader{})
	return c
}

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
		return "video"
	default:
		return ErrUnknownType.Error()
	}
}

// RTPCodec represents a codec supported by the PeerConnection
type RTPCodec struct {
	RTPCodecCapability
	Type        RTPCodecType
	Name        string
	PayloadType uint8
	Payloader   rtp.Payloader
}

// NewRTPCodec is used to define a new codec
func NewRTPCodec(
	codecType RTPCodecType,
	name string,
	clockrate uint32,
	channels uint16,
	fmtp string,
	payloadType uint8,
	payloader rtp.Payloader,
) *RTPCodec {
	return &RTPCodec{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:    codecType.String() + "/" + name,
			ClockRate:   clockrate,
			Channels:    channels,
			SDPFmtpLine: fmtp,
		},
		PayloadType: payloadType,
		Payloader:   payloader,
		Type:        codecType,
		Name:        name,
	}
}

// RTPCodecCapability provides information about codec capabilities.
type RTPCodecCapability struct {
	MimeType    string
	ClockRate   uint32
	Channels    uint16
	SDPFmtpLine string
}

// RTPHeaderExtensionCapability is used to define a RFC5285 RTP header extension supported by the codec.
type RTPHeaderExtensionCapability struct {
	URI string
}

// RTPCapabilities represents the capabilities of a transceiver
type RTPCapabilities struct {
	Codecs           []RTPCodecCapability
	HeaderExtensions []RTPHeaderExtensionCapability
}
