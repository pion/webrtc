package sdp

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
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
	return uint64(rand.Uint32())<<32 + uint64(rand.Uint32())
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
func (sd *SessionDescription) GetCodecForPayloadType(payloadType uint8) (Codec, error) {
	codec := Codec{
		PayloadType: payloadType,
	}

	found := false
	payloadTypeString := strconv.Itoa(int(payloadType))
	rtpmapPrefix := "rtpmap:" + payloadTypeString
	fmtpPrefix := "fmtp:" + payloadTypeString

	for _, m := range sd.MediaDescriptions {
		for _, a := range m.Attributes {
			if strings.HasPrefix(a, rtpmapPrefix) {
				found = true
				// a=rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
				split := strings.Split(a, " ")
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
			} else if strings.HasPrefix(a, fmtpPrefix) {
				// a=fmtp:<format> <format specific parameters>
				split := strings.Split(a, " ")
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
