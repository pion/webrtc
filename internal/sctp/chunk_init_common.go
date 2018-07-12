package sctp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

/*
InitCommon represents an SCTP Chunk body of type INIT and INIT ACK

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
*/

type InitCommon struct {
	initiateTag                    uint32
	advertisedReceiverWindowCredit uint32
	numOutboundStreams             uint16
	numInboundStreams              uint16
	initialTSN                     uint32
	params                         []Param
}

const (
	initChunkMinLength          = 16
	initOptionalVarHeaderLength = 4
)

func getParamPadding(len uint16, multiple uint16) uint16 {
	return (multiple - (len % multiple)) % multiple
}

// Unmarshal populates a Init Chunk from a byte slice
func (i *InitCommon) Unmarshal(raw []byte) error {

	i.initiateTag = binary.BigEndian.Uint32(raw[0:])
	i.advertisedReceiverWindowCredit = binary.BigEndian.Uint32(raw[4:])
	i.numOutboundStreams = binary.BigEndian.Uint16(raw[8:])
	i.numInboundStreams = binary.BigEndian.Uint16(raw[10:])
	i.initialTSN = binary.BigEndian.Uint32(raw[12:])

	// https://tools.ietf.org/html/rfc4960#section-3.2.1
	//
	// Chunk values of SCTP control chunks consist of a chunk-type-specific
	// header of required fields, followed by zero or more parameters.  The
	// optional and variable-length parameters contained in a chunk are
	// defined in a Type-Length-Value format as shown below.
	//
	// 0                   1                   2                   3
	// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |          Parameter Type       |       Parameter Length        |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                                                               |
	// |                       Parameter Value                         |
	// |                                                               |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	offset := initChunkMinLength
	remaining := len(raw) - offset
	for remaining > 0 {
		if remaining > initOptionalVarHeaderLength {
			paramType := ParamType(binary.BigEndian.Uint16(raw[offset:]))
			p, err := BuildParam(paramType, raw[offset:])
			if err != nil {
				return errors.Wrap(err, "Failed unmarshalling param in Init Chunk")
			}
			i.params = append(i.params, p)
			offset += int(p.Length())
			remaining -= int(p.Length())
		} else {
			break
		}
	}

	return nil
}

// Marshal generates raw data from a Init struct
func (i *InitCommon) Marshal() ([]byte, error) {
	out := make([]byte, initChunkMinLength)
	binary.BigEndian.PutUint32(out[0:], i.initiateTag)
	binary.BigEndian.PutUint32(out[4:], i.advertisedReceiverWindowCredit)
	binary.BigEndian.PutUint16(out[8:], i.numOutboundStreams)
	binary.BigEndian.PutUint16(out[10:], i.numInboundStreams)
	binary.BigEndian.PutUint32(out[12:], i.initialTSN)
	for _, p := range i.params {
		pp, err := p.Marshal()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to marshal parameter for INIT/INITACK")
		}
		out = append(out, pp...)
	}

	return out, nil
}
