package ice

import (
	"net"
	"net/url"
	"strconv"
)

// TODO: Migrate address parsing to STUN/TURN packages?

// Scheme indicates the type of server used
type SchemeType int

const (
	// SchemeTypeSTUN indicates the URL represents a STUN server
	SchemeTypeSTUN SchemeType = iota + 1
	SchemeTypeSTUNS

	// SchemeTypeTURN indicates the URL represents a TURN server
	SchemeTypeTURN
	SchemeTypeTURNS
)

func NewSchemeType(raw string) (unknown SchemeType) {
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
		return unknown
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

// Proto indicates the transport that is used
type ProtoType int

const (
	// ProtoTypeUDP indicates the URL uses a UDP transport
	ProtoTypeUDP ProtoType = iota + 1

	// ProtoTypeTCP indicates the URL uses a TCP transport
	ProtoTypeTCP
)

func NewProtoType(raw string) (unknown ProtoType) {
	switch raw {
	case "udp":
		return ProtoTypeUDP
	case "tcp":
		return ProtoTypeTCP
	default:
		return unknown
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

func ParseURL(raw string) (*URL, error) {
	rawParts, err := url.Parse(raw)
	if err != nil {
		return nil, &UnknownError{err}
	}

	var u URL
	u.Scheme = NewSchemeType(rawParts.Scheme)
	if u.Scheme == SchemeType(Unknown) {
		return nil, &SyntaxError{ErrSchemeType}
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
		return nil, &UnknownError{err}
	}

	if u.Host == "" {
		return nil, &SyntaxError{ErrHost}
	}

	if u.Port, err = strconv.Atoi(rawPort); err != nil {
		return nil, &SyntaxError{ErrPort}
	}

	switch {
	case u.Scheme == SchemeTypeSTUN:
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil || (err == nil && len(qArgs) > 0) {
			return nil, &SyntaxError{ErrSTUNQuery}
		}
		u.Proto = ProtoTypeUDP
	case u.Scheme == SchemeTypeSTUNS:
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil || (err == nil && len(qArgs) > 0) {
			return nil, &SyntaxError{ErrSTUNQuery}
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
		return ProtoType(Unknown), &SyntaxError{ErrInvalidQuery}
	}

	var proto ProtoType
	if rawProto := qArgs.Get("transport"); rawProto != "" {
		if proto = NewProtoType(rawProto); proto == ProtoType(0) {
			return ProtoType(Unknown), &NotSupportedError{ErrProtoType}
		}
		return proto, nil
	}

	if len(qArgs) > 0 {
		return ProtoType(Unknown), &SyntaxError{ErrInvalidQuery}
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

func (u URL) IsSecure() bool {
	return u.Scheme == SchemeTypeSTUNS || u.Scheme == SchemeTypeTURNS
}
