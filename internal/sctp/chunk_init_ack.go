package sctp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

/*
InitAck represents an SCTP Chunk of type INIT ACK

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 2    |  Chunk Flags  |      Chunk Length             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         Initiate Tag                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|              Advertised Receiver Window Credit                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Number of Outbound Streams   |  Number of Inbound Streams    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          Initial TSN                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|              Optional/Variable-Length Parameters              |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+


The INIT ACK chunk contains the following parameters.  Unless otherwise
noted, each parameter MUST only be included once in the INIT ACK chunk.

Fixed Parameters                     Status
----------------------------------------------
Initiate Tag                        Mandatory
Advertised Receiver Window Credit   Mandatory
Number of Outbound Streams          Mandatory
Number of Inbound Streams           Mandatory
Initial TSN                         Mandatory

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
	initiateTag                    uint32
	advertisedReceiverWindowCredit uint32
	numOutboundStreams             uint16
	numInboundStreams              uint16
	initialTSN                     uint32
	params                         []Param
}

const (
	initAckChunkMinLength = 16
)

// Unmarshal populates a InitAck Chunk from a byte slice
func (i *InitAck) Unmarshal(raw []byte) error {
	return errors.Errorf("Unimplemented")
}

// Marshal serializes a InitAck to bytes
func (i *InitAck) Marshal() ([]byte, error) {
	out := make([]byte, initAckChunkMinLength)
	binary.BigEndian.PutUint32(out[0:], i.initiateTag)
	binary.BigEndian.PutUint32(out[4:], i.advertisedReceiverWindowCredit)
	binary.BigEndian.PutUint16(out[8:], i.numOutboundStreams)
	binary.BigEndian.PutUint16(out[10:], i.numInboundStreams)
	binary.BigEndian.PutUint32(out[12:], i.initialTSN)

	// TODO fill in params
	return out, nil
}

// Check asserts the validity of this structs values
func (i *InitAck) Check() error {
	return nil
}
