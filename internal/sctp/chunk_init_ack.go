package sctp

import (
	"github.com/pkg/errors"
)

/*
InitAck represents an SCTP Chunk of type INIT ACK

See InitCommon for the fixed headers

Variable Parameters                  Status     Type Value
-------------------------------------------------------------
State Cookie                        Mandatory   7
IPv4 Address (Note 1)               Optional    5
IPv6 Address (Note 1)               Optional    6
Unrecognized Parameter              Optional    8
Reserved for ECN Capable (Note 2)   Optional    32768 (0x8000)
Host Name Address (Note 3)          Optional    11<Paste>

*/
type InitAck struct {
	ChunkHeader
	InitCommon
}

// Unmarshal populates a Init Chunk from a byte slice
func (i *InitAck) Unmarshal(raw []byte) error {
	if err := i.ChunkHeader.Unmarshal(raw); err != nil {
		return err
	}

	if i.typ != INITACK {
		return errors.Errorf("ChunkType is not of type INIT ACK, actually is %s", i.typ.String())
	} else if len(i.Value) < initChunkMinLength {
		return errors.Errorf("Chunk Value isn't long enough for mandatory parameters exp: %d actual: %d", initChunkMinLength, len(i.Value))
	}

	// The Chunk Flags field in INIT is reserved, and all bits in it should
	// be set to 0 by the sender and ignored by the receiver.  The sequence
	// of parameters within an INIT can be processed in any order.
	if i.Flags != 0 {
		return errors.New("ChunkType of type INIT ACK flags must be all 0")
	}

	if err := i.InitCommon.Unmarshal(i.Value); err != nil {
		errors.Wrap(err, "Failed to unmarshal INIT body")
	}

	return nil
}

// Marshal generates raw data from a Init struct
func (i *InitAck) Marshal() ([]byte, error) {
	initShared, err := i.InitCommon.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "Failed marshalling INIT common data")
	}

	i.ChunkHeader.typ = INITACK
	chunkHeader, err := i.ChunkHeader.Marshal(len(initShared))
	if err != nil {
		return nil, errors.Wrap(err, "Failed marshalling InitAck Chunk Header")
	}

	return append(chunkHeader, initShared...), nil
}

// Check asserts the validity of this structs values
func (i *InitAck) Check() (abort bool, err error) {

	// The receiver of the INIT ACK records the value of the Initiate Tag
	// parameter.  This value MUST be placed into the Verification Tag
	// field of every SCTP packet that the INIT ACK receiver transmits
	// within this association.
	//
	// The Initiate Tag MUST NOT take the value 0.  See Section 5.3.1 for
	// more on the selection of the Initiate Tag value.
	//
	// If the value of the Initiate Tag in a received INIT ACK chunk is
	// found to be 0, the receiver MUST destroy the association
	// discarding its TCB.  The receiver MAY send an ABORT for debugging
	// purpose.
	if i.initiateTag == 0 {
		abort = true
		return abort, errors.New("ChunkType of type INIT ACK InitiateTag must not be 0")
	}

	// Defines the maximum number of streams the sender of this INIT ACK
	// chunk allows the peer end to create in this association.  The
	// value 0 MUST NOT be used.
	//
	// Note: There is no negotiation of the actual number of streams but
	// instead the two endpoints will use the min(requested, offered).
	// See Section 5.1.1 for details.
	//
	// Note: A receiver of an INIT ACK with the MIS value set to 0 SHOULD
	// destroy the association discarding its TCB.
	if i.numInboundStreams == 0 {
		abort = true
		return abort, errors.New("INIT ACK inbound stream request must be > 0")
	}

	// Defines the number of outbound streams the sender of this INIT ACK
	// chunk wishes to create in this association.  The value of 0 MUST
	// NOT be used, and the value MUST NOT be greater than the MIS value
	// sent in the INIT chunk.
	//
	// Note: A receiver of an INIT ACK with the OS value set to 0 SHOULD
	// destroy the association discarding its TCB.

	if i.numOutboundStreams == 0 {
		abort = true
		return abort, errors.New("INIT ACK outbound stream request must be > 0")
	}

	return false, nil
}
