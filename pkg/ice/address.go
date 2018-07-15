package ice

import (
	"log"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// TODO: Migrate address parsing to STUN/TURN packages?

var (
	// ErrServerType indicates the server type could not be parsed
	ErrServerType = errors.New("unknown server type")

	// ErrSTUNQuery indicates query arguments are provided in a STUN URL
	ErrSTUNQuery = errors.New("queries not supported in stun address")

	// ErrInvalidQuery indicates an unsupported query is provided
	ErrInvalidQuery = errors.New("invalid query")

	// ErrTransportType indicates an unsupported transport type was provided
	ErrTransportType = errors.New("invalid transport type")

	// ErrHost indicates the server hostname could not be parsed
	ErrHost = errors.New("invalid hostname")

	// ErrPort indicates the server port could not be parsed
	ErrPort = errors.New("invalid port")
)

// ServerType indicates the type of server used
type ServerType int

const (
	// ServerTypeSTUN indicates the URL represents a STUN server
	ServerTypeSTUN ServerType = iota + 1

	// ServerTypeTURN indicates the URL represents a TURN server
	ServerTypeTURN
)

func (t ServerType) String() string {
	switch t {
	case ServerTypeSTUN:
		return "stun"
	case ServerTypeTURN:
		return "turn"
	default:
		return "Unknown"
	}
}

// TransportType indicates the transport that is used
type TransportType int

const (
	// TransportUDP indicates the URL uses a UDP transport
	TransportUDP TransportType = iota + 1

	// TransportTCP indicates the URL uses a TCP transport
	TransportTCP
)

func (t TransportType) String() string {
	switch t {
	case TransportUDP:
		return "udp"
	case TransportTCP:
		return "tcp"
	default:
		return "Unknown"
	}
}

// URL represents a STUN (rfc7064) or TRUN (rfc7065) URL
type URL struct {
	Type          ServerType
	Secure        bool
	Host          string
	Port          int
	TransportType TransportType
}

// NewURL creates a new URL by parsing a STUN (rfc7064) or TRUN (rfc7065) uri string
func NewURL(address string) (URL, error) {
	var result URL

	var scheme string
	scheme, address = split(address, ":")

	switch strings.ToLower(scheme) {
	case "stun":
		result.Type = ServerTypeSTUN
		result.Secure = false

	case "stuns":
		result.Type = ServerTypeSTUN
		result.Secure = true

	case "turn":
		result.Type = ServerTypeTURN
		result.Secure = false

	case "turns":
		result.Type = ServerTypeTURN
		result.Secure = true

	default:
		return result, ErrServerType
	}

	var query string
	address, query = split(address, "?")

	if query != "" {
		if result.Type == ServerTypeSTUN {
			return result, ErrSTUNQuery
		}
		key, value := split(query, "=")
		if strings.ToLower(key) != "transport" {
			return result, ErrInvalidQuery
		}
		switch strings.ToLower(value) {
		case "udp":
			result.TransportType = TransportUDP
		case "tcp":
			result.TransportType = TransportTCP
		default:
			return result, ErrTransportType
		}
	} else {
		if result.Secure {
			result.TransportType = TransportTCP
		} else {
			result.TransportType = TransportUDP
		}
	}

	var host string
	var port string
	colon := strings.IndexByte(address, ':')
	if colon == -1 {
		host = address
		if result.Secure {
			port = "5349"
		} else {
			port = "3478"
		}
	} else if i := strings.IndexByte(address, ']'); i != -1 {
		host = strings.TrimPrefix(address[:i], "[")
		port = address[i+1+len(":"):]
		log.Println(port)
	} else {
		host = address[:colon]
		port = address[colon+len(":"):]
	}
	if host == "" {
		return result, ErrHost
	}
	result.Host = strings.ToLower(host)

	var err error
	result.Port, err = strconv.Atoi(port)
	if err != nil {
		return result, ErrPort
	}

	return result, nil
}

func split(s string, c string) (string, string) {
	i := strings.Index(s, c)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+len(c):]
}
