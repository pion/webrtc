package rtp

type Packet struct {
	Version          uint8
	Padding          bool
	Extension        bool
	Marker           bool
	PayloadType      uint8
	SequenceNumber   uint16
	Timestamp        uint32
	SSRC             uint32
	CSRC             []uint32
	ExtensionProfile uint16
	ExtensionPayload []byte
	Payload          []byte
}

const (
	headerLength    = 4
	versionShift    = 6
	versionMask     = 0x3
	paddingShift    = 5
	paddingMask     = 0x1
	extensionShift  = 4
	extensionMask   = 0x1
	ccMask          = 0xF
	markerShift     = 7
	markerMask      = 0x1
	ptMask          = 0x7F
	seqNumOffset    = 2
	seqNumLength    = 2
	timestampOffset = 4
	timestampLength = 4
	ssrcOffset      = 8
	ssrcLength      = 4
	csrcOffset      = 12
	csrcLength      = 4
)
