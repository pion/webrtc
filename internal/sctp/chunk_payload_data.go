package sctp

import (
	"encoding/binary"
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

	tsn                       uint32
	streamIdentifier          uint16
	streamSequenceNumber      uint16
	payloadProtocolIdentifier uint32
	userData                  []byte
}

const (
	payloadDataEndingFragmentBitmask   = 1
	payloadDataBeginingFragmentBitmask = 2
	payloadDataUnorderedBitmask        = 4
	payloadDataImmediateSACK           = 8

	payloadDataHeaderSize = 12
)

func (p *chunkPayloadData) unmarshal(raw []byte) error {
	if err := p.chunkHeader.unmarshal(raw); err != nil {
		return err
	}

	p.immediateSack = p.flags&payloadDataImmediateSACK != 0
	p.unordered = p.flags&payloadDataUnorderedBitmask != 0
	p.beginingFragment = p.flags&payloadDataBeginingFragmentBitmask != 0
	p.endingFragment = p.flags&payloadDataEndingFragmentBitmask != 0

	p.tsn = binary.BigEndian.Uint32(p.raw[0:])
	p.streamIdentifier = binary.BigEndian.Uint16(p.raw[4:])
	p.streamSequenceNumber = binary.BigEndian.Uint16(p.raw[6:])
	p.payloadProtocolIdentifier = binary.BigEndian.Uint32(p.raw[8:])
	p.userData = p.raw[payloadDataHeaderSize:]

	return nil
}

func (p *chunkPayloadData) marshal() ([]byte, error) {
	return nil, nil
}

func (p *chunkPayloadData) check() (abort bool, err error) {
	return false, nil
}
