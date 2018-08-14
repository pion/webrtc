package sdp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// The "i=" field provides textual information about the session.
type Information string

func (i *Information) String() *string {
	output := string(*i)
	return &output
}

// The "c=" field contains connection data.
type ConnectionInformation struct {
	NetworkType string
	AddressType string
	Address     *Address
}

func (c *ConnectionInformation) String() *string {
	output := fmt.Sprintf(
		"%v %v %v",
		c.NetworkType,
		c.AddressType,
		c.Address.String(),
	)
	return &output
}

// Address desribes a structured address token from within the "c=" field.
type Address struct {
	IP    net.IP
	TTL   *int
	Range *int
}

func (c *Address) String() string {
	var parts []string
	parts = append(parts, c.IP.String())
	if c.TTL != nil {
		parts = append(parts, strconv.Itoa(*c.TTL))
	}

	if c.Range != nil {
		parts = append(parts, strconv.Itoa(*c.Range))
	}

	return strings.Join(parts, "/")
}

// This OPTIONAL field denotes the proposed bandwidth to be used by the
// session or media.
type Bandwidth struct {
	Experimental bool
	Type         string
	Bandwidth    uint64
}

func (b *Bandwidth) String() *string {
	var output string
	if b.Experimental {
		output += "X-"
	}
	output += b.Type + ":" + strconv.FormatUint(b.Bandwidth, 10)
	return &output
}

// If transported over a secure and trusted channel, the Session Description
// Protocol MAY be used to convey encryption keys.
type EncryptionKey string

func (s *EncryptionKey) String() *string {
	output := string(*s)
	return &output
}

// Attributes are the primary means for extending SDP.
type Attribute string

func (a *Attribute) String() *string {
	output := string(*a)
	return &output
}
