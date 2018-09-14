package rtcp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// PacketType specifies the type of an RTCP packet
type PacketType uint8

// RTCP packet types registered with IANA. See: https://www.iana.org/assignments/rtp-parameters/rtp-parameters.xhtml#rtp-parameters-4
const (
	TypeSenderReport       PacketType = 200 // RFC 3550, 6.4.1
	TypeReceiverReport     PacketType = 201 // RFC 3550, 6.4.2
	TypeSourceDescription  PacketType = 202 // RFC 3550, 6.5
	TypeGoodbye            PacketType = 203 // RFC 3550, 6.6
	TypeApplicationDefined PacketType = 204 // RFC 3550, 6.7
)

func (p PacketType) String() string {
	switch p {
	case TypeSenderReport:
		return "SR"
	case TypeReceiverReport:
		return "RR"
	case TypeSourceDescription:
		return "SDES"
	case TypeGoodbye:
		return "BYE"
	case TypeApplicationDefined:
		return "APP"
	default:
		return string(p)
	}
}

const rtpVersion = 2

// A Header is the common header shared by all RTCP packets
type Header struct {
	// Identifies the version of RTP, which is the same in RTCP packets
	// as in RTP data packets.
	Version uint8
	// If the padding bit is set, this individual RTCP packet contains
	// some additional padding octets at the end which are not part of
	// the control information but are included in the length field.
	Padding bool
	// The number of reception reports or sources contained in this packet (depending on the Type)
	Count uint8
	// The RTCP packet type for this packet
	Type PacketType
	// The length of this RTCP packet in 32-bit words minus one,
	// including the header and any padding.
	Length uint16
}

var (
	errInvalidTotalLost = errors.New("rtcp: invalid total lost count")
	errInvalidHeader    = errors.New("rtcp: invalid header")
	errTooManyReports   = errors.New("rtcp: too many reports")
	errTooManyChunks    = errors.New("rtcp: too many chunks")
	errPacketTooShort   = errors.New("rtcp: packet too short")
	errWrongType        = errors.New("rtcp: wrong packet type")
)

const (
	headerLength = 4
	versionShift = 6
	versionMask  = 0x3
	paddingShift = 5
	paddingMask  = 0x1
	countShift   = 0
	countMask    = 0x1f
	countMax     = (1 << 5) - 1
)

// Marshal encodes the Header in binary
func (h Header) Marshal() ([]byte, error) {
	/*
	 *  0                   1                   2                   3
	 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |V=2|P|    RC   |   PT=SR=200   |             length            |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */
	rawPacket := make([]byte, headerLength)

	if h.Version > 3 {
		return nil, errInvalidHeader
	}
	rawPacket[0] |= h.Version << versionShift

	if h.Padding {
		rawPacket[0] |= 1 << paddingShift
	}

	if h.Count > 31 {
		return nil, errInvalidHeader
	}
	rawPacket[0] |= h.Count << countShift

	rawPacket[1] = uint8(h.Type)

	binary.BigEndian.PutUint16(rawPacket[2:], h.Length)

	return rawPacket, nil
}

// Unmarshal decodes the Header from binary
func (h *Header) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < headerLength {
		return errInvalidHeader
	}

	/*
	 *  0                   1                   2                   3
	 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * |V=2|P|    RC   |   PT=SR=200   |             length            |
	 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	h.Version = rawPacket[0] >> versionShift & versionMask
	h.Padding = (rawPacket[0] >> paddingShift & paddingMask) > 0
	h.Count = rawPacket[0] >> countShift & countMask

	h.Type = PacketType(rawPacket[1])

	h.Length = binary.BigEndian.Uint16(rawPacket[2:])

	return nil
}
