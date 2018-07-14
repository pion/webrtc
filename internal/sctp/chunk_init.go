package sctp

import (
	"github.com/pkg/errors"
)

/*
Init represents an SCTP Chunk of type INIT

See InitCommon for the fixed headers

Variable Parameters                  Status     Type Value
-------------------------------------------------------------
IPv4 Address (Note 1)               Optional    5
IPv6 Address (Note 1)               Optional    6
Cookie Preservative                 Optional    9
Reserved for ECN Capable (Note 2)   Optional    32768 (0x8000)
Host Name Address (Note 3)          Optional    11
Supported Address Types (Note 4)    Optional    12
*/
type Init struct {
	ChunkHeader
	InitCommon
}

// Unmarshal populates a Init Chunk from a byte slice
func (i *Init) Unmarshal(raw []byte) error {
	if err := i.ChunkHeader.Unmarshal(raw); err != nil {
		return err
	}

	if i.typ != INIT {
		return errors.Errorf("ChunkType is not of type INIT, actually is %s", i.typ.String())
	} else if len(i.raw) < initChunkMinLength {
		return errors.Errorf("Chunk Value isn't long enough for mandatory parameters exp: %d actual: %d", initChunkMinLength, len(i.raw))
	}

	// The Chunk Flags field in INIT is reserved, and all bits in it should
	// be set to 0 by the sender and ignored by the receiver.  The sequence
	// of parameters within an INIT can be processed in any order.
	if i.Flags != 0 {
		return errors.New("ChunkType of type INIT flags must be all 0")
	}

	if err := i.InitCommon.Unmarshal(i.raw); err != nil {
		errors.Wrap(err, "Failed to unmarshal INIT body")
	}

	return nil
}

// Marshal generates raw data from a Init struct
func (i *Init) Marshal() ([]byte, error) {
	initShared, err := i.InitCommon.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "Failed marshalling INIT common data")
	}

	i.ChunkHeader.typ = INIT
	i.ChunkHeader.raw = initShared
	return i.ChunkHeader.Marshal()
}

// Check asserts the validity of this structs values
func (i *Init) Check() (abort bool, err error) {
	// The receiver of the INIT (the responding end) records the value of
	// the Initiate Tag parameter.  This value MUST be placed into the
	// Verification Tag field of every SCTP packet that the receiver of
	// the INIT transmits within this association.
	//
	// The Initiate Tag is allowed to have any value except 0.  See
	// Section 5.3.1 for more on the selection of the tag value.
	//
	// If the value of the Initiate Tag in a received INIT chunk is found
	// to be 0, the receiver MUST treat it as an error and close the
	// association by transmitting an ABORT.
	if i.initiateTag == 0 {
		abort = true
		return abort, errors.New("ChunkType of type INIT ACK InitiateTag must not be 0")
	}

	// Defines the maximum number of streams the sender of this INIT
	// chunk allows the peer end to create in this association.  The
	// value 0 MUST NOT be used.
	//
	// Note: There is no negotiation of the actual number of streams but
	// instead the two endpoints will use the min(requested, offered).
	// See Section 5.1.1 for details.
	//
	// Note: A receiver of an INIT with the MIS value of 0 SHOULD abort
	// the association.
	if i.numInboundStreams == 0 {
		abort = true
		return abort, errors.New("INIT inbound stream request must be > 0")
	}

	// Defines the number of outbound streams the sender of this INIT
	// chunk wishes to create in this association.  The value of 0 MUST
	// NOT be used.
	//
	// Note: A receiver of an INIT with the OS value set to 0 SHOULD
	// abort the association.

	if i.numOutboundStreams == 0 {
		abort = true
		return abort, errors.New("INIT outbound stream request must be > 0")
	}

	return false, nil
}
