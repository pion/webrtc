package sdp

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"

	"time"

	"github.com/pkg/errors"
)

// ConnectionRole indicates which of the end points should initiate the connection establishment
type ConnectionRole int

const (
	// ConnectionRoleActive indicates the endpoint will initiate an outgoing connection.
	ConnectionRoleActive ConnectionRole = iota + 1

	// ConnectionRolePassive indicates the endpoint will accept an incoming connection.
	ConnectionRolePassive

	// ConnectionRoleActpass indicates the endpoint is willing to accept an incoming connection or to initiate an outgoing connection.
	ConnectionRoleActpass

	// ConnectionRoleHoldconn indicates the endpoint does not want the connection to be established for the time being.
	ConnectionRoleHoldconn
)

func (t ConnectionRole) String() string {
	switch t {
	case ConnectionRoleActive:
		return "active"
	case ConnectionRolePassive:
		return "passive"
	case ConnectionRoleActpass:
		return "actpass"
	case ConnectionRoleHoldconn:
		return "holdconn"
	default:
		return "Unknown"
	}
}

func newSessionID() uint64 {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return uint64(r.Uint32()*2) >> 2
}

// Codec represents a codec
type Codec struct {
	PayloadType        uint8
	Name               string
	ClockRate          uint32
	EncodingParameters string
	Fmtp               string
}

func (c Codec) String() string {
	return fmt.Sprintf("%d %s/%d/%s", c.PayloadType, c.Name, c.ClockRate, c.EncodingParameters)
}

// GetCodecForPayloadType scans the SessionDescription for the given payloadType and returns the codec
func (s *SessionDescription) GetCodecForPayloadType(payloadType uint8) (Codec, error) {
	codec := Codec{
		PayloadType: payloadType,
	}

	found := false
	payloadTypeString := strconv.Itoa(int(payloadType))
	rtpmapPrefix := "rtpmap:" + payloadTypeString
	fmtpPrefix := "fmtp:" + payloadTypeString

	for _, m := range s.MediaDescriptions {
		for _, a := range m.Attributes {
			if strings.HasPrefix(*a.String(), rtpmapPrefix) {
				found = true
				// a=rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
				split := strings.Split(*a.String(), " ")
				if len(split) == 2 {
					split = strings.Split(split[1], "/")
					codec.Name = split[0]
					parts := len(split)
					if parts > 1 {
						rate, err := strconv.Atoi(split[1])
						if err != nil {
							return codec, err
						}
						codec.ClockRate = uint32(rate)
					}
					if parts > 2 {
						codec.EncodingParameters = split[2]
					}
				}
			} else if strings.HasPrefix(*a.String(), fmtpPrefix) {
				// a=fmtp:<format> <format specific parameters>
				split := strings.Split(*a.String(), " ")
				if len(split) == 2 {
					codec.Fmtp = split[1]
				}
			}
		}
		if found {
			return codec, nil
		}
	}
	return codec, errors.New("payload type not found")
}

type lexer struct {
	desc  *SessionDescription
	input *bufio.Reader
}

type stateFn func(*lexer) (stateFn, error)

func readType(input *bufio.Reader) (string, error) {
	key, err := input.ReadString('=')
	if err != nil {
		return key, err
	}

	if len(key) != 2 {
		return key, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	return key, nil
}

func readValue(input *bufio.Reader) (string, error) {
	line, err := input.ReadString('\n')
	if err != nil && err != io.EOF {
		return line, err
	}

	if len(line) == 0 {
		return line, nil
	}

	if line[len(line)-1] == '\n' {
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			drop = 2
		}
		line = line[:len(line)-drop]
	}

	return line, nil
}

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1
}

func keyValueBuild(key string, value *string) string {
	if value != nil {
		return key + *value + "\r\n"
	}
	return ""
}
