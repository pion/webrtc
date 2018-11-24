package sdp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Information describes the "i=" field which provides textual information
// about the session.
type Information string

func (i *Information) String() *string {
	output := string(*i)
	return &output
}

// ConnectionInformation defines the representation for the "c=" field
// containing connection data.
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

// Bandwidth describes an optional field which denotes the proposed bandwidth
// to be used by the session or media.
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

// EncryptionKey describes the "k=" which conveys encryption key information.
type EncryptionKey string

func (s *EncryptionKey) String() *string {
	output := string(*s)
	return &output
}

// Attribute describes the "a=" field which represents the primary means for
// extending SDP.
type Attribute struct {
	Key   string
	Value string
}

// NewPropertyAttribute constructs a new attribute
func NewPropertyAttribute(key string) Attribute {
	return Attribute{
		Key: key,
	}
}

// NewAttribute constructs a new attribute
func NewAttribute(key, value string) Attribute {
	return Attribute{
		Key:   key,
		Value: value,
	}
}

func (a *Attribute) String() *string {
	output := a.Key
	if len(a.Value) > 0 {
		output += ":" + a.Value
	}
	return &output
}
