package sctp

import (
	"encoding/binary"
	"fmt"
)

/*
chunkPayloadData represents an SCTP Chunk of type DATA

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 0    | Reserved|U|B|E|    Length                     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              TSN                              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      Stream Identifier S      |   Stream Sequence Number n    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  Payload Protocol Identifier                  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                 User Data (seq n of Stream S)                 |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+


An unfragmented user message shall have both the B and E bits set to
'1'.  Setting both B and E bits to '0' indicates a middle fragment of
a multi-fragment user message, as summarized in the following table:
   B E                  Description
============================================================
|  1 0 | First piece of a fragmented user message          |
+----------------------------------------------------------+
|  0 0 | Middle piece of a fragmented user message         |
+----------------------------------------------------------+
|  0 1 | Last piece of a fragmented user message           |
+----------------------------------------------------------+
|  1 1 | Unfragmented message                              |
============================================================
|             Table 1: Fragment Description Flags          |
============================================================
*/
type chunkPayloadData struct {
	chunkHeader

	unordered        bool
	beginingFragment bool
	endingFragment   bool
	immediateSack    bool

	tsn                  uint32
	streamID             uint16
	streamSequenceNumber uint16
	payloadProtocolID    PayloadProtocolID
	userData             []byte
}

const (
	payloadDataEndingFragmentBitmask   = 1
	payloadDataBeginingFragmentBitmask = 2
	payloadDataUnorderedBitmask        = 4
	payloadDataImmediateSACK           = 8

	payloadDataHeaderSize = 12
)

// PayloadProtocolID is an enum for DataChannel payload types
type PayloadProtocolID uint32

// PayloadProtocolID enums
const (
	PayloadTypeWebRTCDcep        PayloadProtocolID = 50
	PayloadTypeWebRTCString      PayloadProtocolID = 51
	PayloadTypeWebRTCBinary      PayloadProtocolID = 53
	PayloadTypeWebRTCStringEmpty PayloadProtocolID = 56
	PayloadTypeWebRTCBinaryEmpty PayloadProtocolID = 57
)

func (p PayloadProtocolID) String() string {
	switch p {
	case PayloadTypeWebRTCDcep:
		return "WebRTC DCEP"
	case PayloadTypeWebRTCString:
		return "WebRTC String"
	case PayloadTypeWebRTCBinary:
		return "WebRTC Binary"
	case PayloadTypeWebRTCStringEmpty:
		return "WebRTC String (Empty)"
	case PayloadTypeWebRTCBinaryEmpty:
		return "WebRTC Binary (Empty)"
	default:
		return fmt.Sprintf("Unknown Payload Protocol Identifier: %d", p)
	}
}

func (p *chunkPayloadData) unmarshal(raw []byte) error {
	if err := p.chunkHeader.unmarshal(raw); err != nil {
		return err
	}

	p.immediateSack = p.flags&payloadDataImmediateSACK != 0
	p.unordered = p.flags&payloadDataUnorderedBitmask != 0
	p.beginingFragment = p.flags&payloadDataBeginingFragmentBitmask != 0
	p.endingFragment = p.flags&payloadDataEndingFragmentBitmask != 0

	p.tsn = binary.BigEndian.Uint32(p.raw[0:])
	p.streamID = binary.BigEndian.Uint16(p.raw[4:])
	p.streamSequenceNumber = binary.BigEndian.Uint16(p.raw[6:])
	p.payloadProtocolID = PayloadProtocolID(binary.BigEndian.Uint32(p.raw[8:]))
	p.userData = p.raw[payloadDataHeaderSize:]

	return nil
}

func (p *chunkPayloadData) marshal() ([]byte, error) {

	payRaw := make([]byte, payloadDataHeaderSize+len(p.userData))

	binary.BigEndian.PutUint32(payRaw[0:], p.tsn)
	binary.BigEndian.PutUint16(payRaw[4:], p.streamID)
	binary.BigEndian.PutUint16(payRaw[6:], p.streamSequenceNumber)
	binary.BigEndian.PutUint32(payRaw[8:], uint32(p.payloadProtocolID))
	copy(payRaw[payloadDataHeaderSize:], p.userData)

	flags := uint8(0)
	if p.endingFragment {
		flags = 1
	}
	if p.beginingFragment {
		flags |= 1 << 1
	}
	if p.unordered {
		flags |= 1 << 2
	}
	if p.immediateSack {
		flags |= 1 << 3
	}

	p.chunkHeader.flags = flags
	p.chunkHeader.typ = PAYLOADDATA
	p.chunkHeader.raw = payRaw
	return p.chunkHeader.marshal()
}

func (p *chunkPayloadData) check() (abort bool, err error) {
	return false, nil
}
