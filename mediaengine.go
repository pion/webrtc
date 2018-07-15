package webrtc

import (
	"strconv"

	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pions/webrtc/pkg/rtp/codecs"
	"github.com/pkg/errors"
)

// PayloadTypes for the default codecs
const (
	PayloadTypeOpus = 111
	PayloadTypeVP8  = 96
	PayloadTypeVP9  = 98
	PayloadTypeH264 = 100
)

// Names for the default codecs
const (
	Opus = "opus"
	VP8  = "VP8"
	VP9  = "VP9"
	H264 = "H264"
)

var rtcMediaEngine = &mediaEngine{}

// RegisterDefaultCodecs is a helper that registers the default codecs supported by pions-webrtc
func RegisterDefaultCodecs() {
	RegisterCodec(NewRTCRtpOpusCodec(PayloadTypeOpus, 48000, 2))
	RegisterCodec(NewRTCRtpVP8Codec(PayloadTypeVP8, 90000))
	RegisterCodec(NewRTCRtpH264Codec(PayloadTypeH264, 90000))
	RegisterCodec(NewRTCRtpVP9Codec(PayloadTypeVP9, 90000))
}

// RegisterCodec is used to register a codec
func RegisterCodec(codec *RTCRtpCodec) {
	rtcMediaEngine.RegisterCodec(codec)
}

type mediaEngine struct {
	codecs []*RTCRtpCodec
}

func (m *mediaEngine) RegisterCodec(codec *RTCRtpCodec) uint8 {
	// TODO: generate PayloadType if not set
	m.codecs = append(m.codecs, codec)
	return codec.PayloadType
}

func (m *mediaEngine) ClearCodecs() {
	m.codecs = nil
}

func (m *mediaEngine) getCodec(payloadType uint8) (*RTCRtpCodec, error) {
	for _, codec := range m.codecs {
		if codec.PayloadType == payloadType {
			return codec, nil
		}
	}
	return nil, errors.New("Codec not found")
}

func (m *mediaEngine) getCodecSDP(sdpCodec sdp.Codec) (*RTCRtpCodec, error) {
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

func (m *mediaEngine) getCodecsByKind(kind RTCRtpCodecType) []*RTCRtpCodec {
	var codecs []*RTCRtpCodec
	for _, codec := range rtcMediaEngine.codecs {
		if codec.Type == kind {
			codecs = append(codecs, codec)
		}
	}
	return codecs
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
		return "Unknown"
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
	typ RTCRtpCodecType,
	name string,
	clockrate uint32,
	channels uint16,
	fmtp string,
	payloadType uint8,
	payloader rtp.Payloader,
) *RTCRtpCodec {
	return &RTCRtpCodec{
		RTCRtpCodecCapability: RTCRtpCodecCapability{
			MimeType:    typ.String() + "/" + name,
			ClockRate:   clockrate,
			Channels:    channels,
			SdpFmtpLine: fmtp,
		},
		PayloadType: payloadType,
		Payloader:   payloader,
		Type:        typ,
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
