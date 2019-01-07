package mux

import (
	"bytes"
	"encoding/binary"
)

// MatchFunc allows custom logic for mapping packets to an Endpoint
type MatchFunc func([]byte) bool

// MatchAll always returns true
func MatchAll(b []byte) bool {
	return true
}

// MatchRange is a MatchFunc that accepts packets with the first byte in [lower..upper]
func MatchRange(lower, upper byte) MatchFunc {
	return func(buf []byte) bool {
		if len(buf) < 1 {
			return false
		}
		b := buf[0]
		return b >= lower && b <= upper
	}
}

// MatchFuncs as described in RFC7983
// https://tools.ietf.org/html/rfc7983
//              +----------------+
//              |        [0..3] -+--> forward to STUN
//              |                |
//              |      [16..19] -+--> forward to ZRTP
//              |                |
//  packet -->  |      [20..63] -+--> forward to DTLS
//              |                |
//              |      [64..79] -+--> forward to TURN Channel
//              |                |
//              |    [128..191] -+--> forward to RTP/RTCP
//              +----------------+

// MatchSTUN is a MatchFunc that accepts packets with the first byte in [0..3]
// as defied in RFC7983
var MatchSTUN = MatchRange(0, 3)

// MatchZRTP is a MatchFunc that accepts packets with the first byte in [16..19]
// as defied in RFC7983
var MatchZRTP = MatchRange(16, 19)

// MatchDTLS is a MatchFunc that accepts packets with the first byte in [20..63]
// as defied in RFC7983
var MatchDTLS = MatchRange(20, 63)

// MatchTURN is a MatchFunc that accepts packets with the first byte in [64..79]
// as defied in RFC7983
var MatchTURN = MatchRange(64, 79)

// MatchSRTPOrSRTCP is a MatchFunc that accepts packets with the first byte in [128..191]
// as defied in RFC7983
var MatchSRTPOrSRTCP = MatchRange(128, 191)

func isRTCP(buf []byte) bool {
	// Not long enough to determine RTP/RTCP
	if len(buf) < 4 {
		return false
	}

	var rtcpPacketType uint8
	r := bytes.NewReader([]byte{buf[1]})
	if err := binary.Read(r, binary.BigEndian, &rtcpPacketType); err != nil {
		return false
	} else if rtcpPacketType >= 192 && rtcpPacketType <= 223 {
		return true
	}

	return false
}

// MatchSRTP is a MatchFunc that only matches SRTP and not SRTCP
var MatchSRTP = func(buf []byte) bool {
	return MatchSRTPOrSRTCP(buf) && !isRTCP(buf)
}

// MatchSRTCP is a MatchFunc that only matches SRTCP and not SRTP
var MatchSRTCP = func(buf []byte) bool {
	return MatchSRTPOrSRTCP(buf) && isRTCP(buf)
}
