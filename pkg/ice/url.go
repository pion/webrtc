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
		return nil, UnknownError{Err: err}
	}

	var u URL
	u.Scheme = NewSchemeType(rawParts.Scheme)
	if u.Scheme == SchemeType(0) {
		return nil, SyntaxError{Err: ErrSchemeType}
	}

	var rawPort string
	u.Host, rawPort, err = net.SplitHostPort(rawParts.Opaque)
	if err != nil {
		switch e := err.(type) {
		case *net.AddrError:
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
			return nil, SyntaxError{Err: ErrHost}
		case *strconv.NumError:
			return nil, SyntaxError{Err: ErrPort}
		default:
			return nil, UnknownError{Err: err}
		}
	}

	if u.Port, err = strconv.Atoi(rawPort); err != nil {
		return nil, SyntaxError{Err: ErrPort}
	}

	switch {
	case u.Scheme == SchemeTypeSTUN:
		u.Proto = ProtoTypeUDP
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil || (err == nil && len(qArgs) > 0) {
			return nil, SyntaxError{Err: ErrSTUNQuery}
		}
	case u.Scheme == SchemeTypeSTUNS:
		u.Proto = ProtoTypeTCP
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil || (err == nil && len(qArgs) > 0) {
			return nil, SyntaxError{Err: ErrSTUNQuery}
		}
	case u.Scheme == SchemeTypeTURN:
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil {
			return nil, SyntaxError{Err: ErrInvalidQuery}
		}
		if proto := qArgs.Get("transport"); proto != "" {
			if u.Proto = NewProtoType(proto); u.Proto == ProtoType(0) {
				return nil, SyntaxError{Err: ErrProtoType}
			}
			break
		}
		u.Proto = ProtoTypeUDP
	case u.Scheme == SchemeTypeTURNS:
		qArgs, err := url.ParseQuery(rawParts.RawQuery)
		if err != nil {
			return nil, SyntaxError{Err: ErrInvalidQuery}
		}
		if proto := qArgs.Get("transport"); proto != "" {
			if u.Proto = NewProtoType(proto); u.Proto == ProtoType(0) {
				return nil, SyntaxError{Err: ErrProtoType}
			}
			break
		}
		u.Proto = ProtoTypeTCP
	}
	return &u, nil
}

func (u URL) IsSecure() bool {
	return u.Scheme == SchemeTypeSTUNS || u.Scheme == SchemeTypeTURNS
}
