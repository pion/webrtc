package ice

import (
	"net"
	"net/url"
	"strconv"

	"github.com/pions/webrtc/pkg/rtcerr"
)

// TODO: Migrate address parsing to STUN/TURN

// SchemeType indicates the type of server used in the ice.URL structure.
type SchemeType int

// Unknown defines default public constant to use for "enum" like struct
// comparisons when no value was defined.
const Unknown = iota

const (
	// SchemeTypeSTUN indicates the URL represents a STUN server.
	SchemeTypeSTUN SchemeType = iota + 1

	// SchemeTypeSTUNS indicates the URL represents a STUNS (secure) server.
	SchemeTypeSTUNS

	// SchemeTypeTURN indicates the URL represents a TURN server.
	SchemeTypeTURN

	// SchemeTypeTURNS indicates the URL represents a TURNS (secure) server.
	SchemeTypeTURNS
)

// NewSchemeType defines a procedure for creating a new SchemeType from a raw
// string naming the scheme type.
func NewSchemeType(raw string) SchemeType {
	switch raw {
	case "stun":
		return SchemeTypeSTUN
	case "stuns":
		return SchemeTypeSTUNS
	case "turn":
		return SchemeTypeTURN
	case "turns":
		return SchemeTypeTURNS
	default:
		return SchemeType(Unknown)
	}
}

func (t SchemeType) String() string {
	switch t {
	case SchemeTypeSTUN:
		return "stun"
	case SchemeTypeSTUNS:
		return "stuns"
	case SchemeTypeTURN:
		return "turn"
	case SchemeTypeTURNS:
		return "turns"
	default:
		return ErrUnknownType.Error()
	}
}

// ProtoType indicates the transport protocol type that is used in the ice.URL
// structure.
type ProtoType int

const (
	// ProtoTypeUDP indicates the URL uses a UDP transport.
	ProtoTypeUDP ProtoType = iota + 1

	// ProtoTypeTCP indicates the URL uses a TCP transport.
	ProtoTypeTCP
)

// NewProtoType defines a procedure for creating a new ProtoType from a raw
// string naming the transport protocol type.
func NewProtoType(raw string) ProtoType {
	switch raw {
	case "udp":
		return ProtoTypeUDP
	case "tcp":
		return ProtoTypeTCP
	default:
		return ProtoType(Unknown)
	}
}

func (t ProtoType) String() string {
	switch t {
	case ProtoTypeUDP:
		return "udp"
	case ProtoTypeTCP:
		return "tcp"
	default:
		return ErrUnknownType.Error()
	}
}

// URL represents a STUN (rfc7064) or TURN (rfc7065) URL
type URL struct {
	Scheme SchemeType
	Host   string
	Port   int
	Proto  ProtoType
}

// ParseURL parses a STUN or TURN urls following the ABNF syntax described in
// https://tools.ietf.org/html/rfc7064 and https://tools.ietf.org/html/rfc7065
// respectively.
func ParseURL(raw string) (*URL, error) {
	rawParts, err := url.Parse(raw)
	if err != nil {
		return nil, &rtcerr.UnknownError{Err: err}
	}

	var u URL
	u.Scheme = NewSchemeType(rawParts.Scheme)
	if u.Scheme == SchemeType(Unknown) {
		return nil, &rtcerr.SyntaxError{Err: ErrSchemeType}
	}

	var rawPort string
	if u.Host, rawPort, err = net.SplitHostPort(rawParts.Opaque); err != nil {
		if e, ok := err.(*net.AddrError); ok {
			if e.Err == "missing port in address" {
				nextRawURL := u.Scheme.String() + ":" + rawParts.Opaque
				switch {
				case u.Scheme == SchemeTypeSTUN || u.Scheme == SchemeTypeTURN:
					nextRawURL += ":3478"
					if rawParts.RawQuery != "" {
						nextRawURL += "?" + rawParts.RawQuery
					}
					return ParseURL(nextRawURL)
				case u.Scheme == SchemeTypeSTUNS || u.Scheme == SchemeTypeTURNS:
					nextRawURL += ":5349"
					if rawParts.RawQuery != "" {
						nextRawURL += "?" + rawParts.RawQuery
					}
					return ParseURL(nextRawURL)
				}
			}
		}
		return nil, &rtcerr.UnknownError{Err: err}
	}

	if u.Host == "" {
		return nil, &rtcerr.SyntaxError{Err: ErrHost}
	}

	if u.Port, err = strconv.Atoi(rawPort); err != nil {
		return nil, &rtcerr.SyntaxError{Err: ErrPort}
	}

	switch {
	case u.Scheme == SchemeTypeSTUN:
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil || (err == nil && len(qArgs) > 0) {
			return nil, &rtcerr.SyntaxError{Err: ErrSTUNQuery}
		}
		u.Proto = ProtoTypeUDP
	case u.Scheme == SchemeTypeSTUNS:
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil || (err == nil && len(qArgs) > 0) {
			return nil, &rtcerr.SyntaxError{Err: ErrSTUNQuery}
		}
		u.Proto = ProtoTypeTCP
	case u.Scheme == SchemeTypeTURN:
		proto, err := parseProto(rawParts.RawQuery)
		if err != nil {
			return nil, err
		}

		u.Proto = proto
		if u.Proto == ProtoType(Unknown) {
			u.Proto = ProtoTypeUDP
		}
	case u.Scheme == SchemeTypeTURNS:
		proto, err := parseProto(rawParts.RawQuery)
		if err != nil {
			return nil, err
		}

		u.Proto = proto
		if u.Proto == ProtoType(Unknown) {
			u.Proto = ProtoTypeTCP
		}
	}

	return &u, nil
}

func parseProto(raw string) (ProtoType, error) {
	qArgs, err := url.ParseQuery(raw)
	if err != nil || len(qArgs) > 1 {
		return ProtoType(Unknown), &rtcerr.SyntaxError{Err: ErrInvalidQuery}
	}

	var proto ProtoType
	if rawProto := qArgs.Get("transport"); rawProto != "" {
		if proto = NewProtoType(rawProto); proto == ProtoType(0) {
			return ProtoType(Unknown), &rtcerr.NotSupportedError{Err: ErrProtoType}
		}
		return proto, nil
	}

	if len(qArgs) > 0 {
		return ProtoType(Unknown), &rtcerr.SyntaxError{Err: ErrInvalidQuery}
	}

	return proto, nil
}

func (u URL) String() string {
	rawURL := u.Scheme.String() + ":" + net.JoinHostPort(u.Host, strconv.Itoa(u.Port))
	if u.Scheme == SchemeTypeTURN || u.Scheme == SchemeTypeTURNS {
		rawURL += "?transport=" + u.Proto.String()
	}
	return rawURL
}

// IsSecure returns whether the this URL's scheme describes secure scheme or not.
func (u URL) IsSecure() bool {
	return u.Scheme == SchemeTypeSTUNS || u.Scheme == SchemeTypeTURNS
}
