package sdp

import (
	"strings"
	"strconv"
)

type MediaDescription struct {
	// m=<media> <port>/<number of ports> <proto> <fmt> ...
	// https://tools.ietf.org/html/rfc4566#section-5.14
	MediaName MediaName

	// i=<session description>
	// https://tools.ietf.org/html/rfc4566#section-5.4
	MediaTitle *Information

	// c=<nettype> <addrtype> <connection-address>
	// https://tools.ietf.org/html/rfc4566#section-5.7
	ConnectionInformation *ConnectionInformation

	// b=<bwtype>:<bandwidth>
	// https://tools.ietf.org/html/rfc4566#section-5.8
	Bandwidth []Bandwidth

	// k=<method>
	// k=<method>:<encryption key>
	// https://tools.ietf.org/html/rfc4566#section-5.12
	EncryptionKey *EncryptionKey

	// a=<attribute>
	// a=<attribute>:<value>
	// https://tools.ietf.org/html/rfc4566#section-5.13
	Attributes []Attribute
}

type RangedPort struct {
	Value int
	Range *int
}

func (p *RangedPort) String() string {
	output := strconv.Itoa(p.Value)
	if p.Range != nil {
		output += "/" + strconv.Itoa(*p.Range)
	}
	return output
}

type MediaName struct {
	Media   string
	Port    RangedPort
	Protos  []string
	Formats []int
}

func (m *MediaName) String() *string {
	var formats []string
	for _, format := range m.Formats {
		formats = append(formats, strconv.Itoa(format))
	}

	output := strings.Join([]string{
		m.Media,
		m.Port.String(),
		strings.Join(m.Protos, "/"),
		strings.Join(formats, ""),
	}, " ")
	return &output
}