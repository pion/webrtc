package sctp

import (
	"encoding/binary"

	"fmt"
	"github.com/pkg/errors"
)

/*
Init represents an SCTP Chunk of type INIT

 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Type = 1    |  Chunk Flags  |      Chunk Length             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         Initiate Tag                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           Advertised Receiver Window Credit (a_rwnd)          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Number of Outbound Streams   |  Number of Inbound Streams    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          Initial TSN                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|              Optional/Variable-Length Parameters              |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

The INIT chunk contains the following parameters.  Unless otherwise
noted, each parameter MUST only be included once in the INIT chunk.

Fixed Parameters                     Status
----------------------------------------------
Initiate Tag                        Mandatory
Advertised Receiver Window Credit   Mandatory
Number of Outbound Streams          Mandatory
Number of Inbound Streams           Mandatory
Initial TSN                         Mandatory

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
	initiateTag                    uint32
	advertisedReceiverWindowCredit uint32
	numOutboundStreams             uint16
	numInboundStreams              uint16
	initialTSN                     uint32
}

const (
	initChunkMinLength          = 16
	initOptionalVarHeaderLength = 4
)

func getParamPadding(len uint16, multiple uint16) uint16 {
	return (multiple - (len % multiple)) % multiple
}

// Unmarshal populates a Init Chunk from a byte slice
func (i *Init) Unmarshal(raw []byte) error {
	if err := i.unmarshalHeader(raw); err != nil {
		return err
	}

	if i.Type != INIT {
		return errors.Errorf("ChunkType is not of type INIT, actually is %s", i.Type.String())
	} else if len(i.Value) < initChunkMinLength {
		return errors.Errorf("Chunk Value isn't long enough for mandatory parameters exp: %d actual: %d", initChunkMinLength, len(i.Value))
	}

	// The Chunk Flags field in INIT is reserved, and all bits in it should
	// be set to 0 by the sender and ignored by the receiver.  The sequence
	// of parameters within an INIT can be processed in any order.
	if i.Flags != 0 {
		return errors.New("ChunkType of type INIT flags must be all 0")
	}

	i.initiateTag = binary.BigEndian.Uint32(i.Value[0:])
	i.advertisedReceiverWindowCredit = binary.BigEndian.Uint32(i.Value[4:])
	i.numOutboundStreams = binary.BigEndian.Uint16(i.Value[8:])
	i.numInboundStreams = binary.BigEndian.Uint16(i.Value[10:])
	i.initialTSN = binary.BigEndian.Uint32(i.Value[12:])

	/*
		https://tools.ietf.org/html/rfc4960#section-3.2.1
		Chunk values of SCTP control chunks consist of a chunk-type-specific
		header of required fields, followed by zero or more parameters.  The
		optional and variable-length parameters contained in a chunk are
		defined in a Type-Length-Value format as shown below.

		0                   1                   2                   3
		0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
		+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		|          Parameter Type       |       Parameter Length        |
		+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		|                                                               |
		|                       Parameter Value                         |
		|                                                               |
		+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	*/

	fmt.Printf("Packet %v\n", i.Value)
	offset := initChunkMinLength
	remaining := len(i.Value) - offset
	for remaining >= 0 {
		if remaining > initOptionalVarHeaderLength {
			paramType := ParamType(binary.BigEndian.Uint16(i.Value[offset:]))
			paramLength := binary.BigEndian.Uint16(i.Value[offset+2:])
			paramLengthPlusPadding := paramLength + getParamPadding(paramLength, 4)
			fmt.Printf("Param Type: %v:, Param Length: %v\n", paramType, paramLength)
			offset += int(paramLengthPlusPadding)
			remaining -= int(paramLengthPlusPadding)
		} else {
			break
		}
	}
	// TODO Sean-Der
	// offset := initChunkMinLength
	// for {
	// 	remaining := len(i.Value) - offset
	// 	if remaining == 0 {
	// 		break
	// 	} else if remaining < initOptionalVarHeaderLength {
	// 		return errors.Errorf("%d bytes remain in init chunk value, not enough to build optional var header", remaining)
	// 	}

	// 	attributeType := binary.BigEndian.Uint16(i.Value[offset:])
	// 	attributeLength := int(binary.BigEndian.Uint16(i.Value[offset+2:]))

	// 	offset += attributeLength
	// }
	return nil
}

// Marshal generates raw data from a Init struct
func (i *Init) Marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}
