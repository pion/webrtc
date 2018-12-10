package mux

// MatchFunc allows custom logic for mapping packets to an Endpoint
type MatchFunc func([]byte) bool

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

// MatchSRTP is a MatchFunc that accepts packets with the first byte in [128..191]
// as defied in RFC7983
var MatchSRTP = MatchRange(128, 191)
