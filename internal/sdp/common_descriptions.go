package sdp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Information string

func (i *Information) String() *string {
	output := string(*i)
	return &output
}

type ConnectionInformation struct {
	NetworkType string
	AddressType string
	Address     *ConnectionAddress
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

type ConnectionAddress struct {
	IP    net.IP
	Ttl   *int
	Range *int
}

func (c *ConnectionAddress) String() string {
	var parts []string
	parts = append(parts, c.IP.String())
	if c.Ttl != nil {
		parts = append(parts, strconv.Itoa(*c.Ttl))
	}

	if c.Range != nil {
		parts = append(parts, strconv.Itoa(*c.Range))
	}

	return strings.Join(parts, "/")
}

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

type EncryptionKey string

func (s *EncryptionKey) String() *string {
	output := string(*s)
	return &output
}

type Attribute string

func (a *Attribute) String() *string {
	output := string(*a)
	return &output
}