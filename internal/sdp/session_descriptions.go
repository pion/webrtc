package sdp

import (
	"fmt"
	"strconv"
	"net/url"
)

type SessionDescription struct {
	// v=0
	// https://tools.ietf.org/html/rfc4566#section-5.1
	ProtocolVersion ProtocolVersion

	// o=<username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
	// https://tools.ietf.org/html/rfc4566#section-5.2
	Origin Origin

	// s=<session name>
	// https://tools.ietf.org/html/rfc4566#section-5.3
	SessionName SessionName

	// i=<session description>
	// https://tools.ietf.org/html/rfc4566#section-5.4
	SessionInformation *Information

	// u=<uri>
	// https://tools.ietf.org/html/rfc4566#section-5.5
	URI *URI

	// e=<email-address>
	// https://tools.ietf.org/html/rfc4566#section-5.6
	EmailAddress *EmailAddress

	// p=<phone-number>
	// https://tools.ietf.org/html/rfc4566#section-5.6
	PhoneNumber *PhoneNumber

	// c=<nettype> <addrtype> <connection-address>
	// https://tools.ietf.org/html/rfc4566#section-5.7
	ConnectionInformation *ConnectionInformation

	// b=<bwtype>:<bandwidth>
	// https://tools.ietf.org/html/rfc4566#section-5.8
	Bandwidth []Bandwidth

	// https://tools.ietf.org/html/rfc4566#section-5.9
	// https://tools.ietf.org/html/rfc4566#section-5.10
	TimeDescriptions []TimeDescription

	// z=<adjustment time> <offset> <adjustment time> <offset> ...
	// https://tools.ietf.org/html/rfc4566#section-5.11
	TimeZones []TimeZone

	// k=<method>
	// k=<method>:<encryption key>
	// https://tools.ietf.org/html/rfc4566#section-5.12
	EncryptionKey EncryptionKey

	// a=<attribute>
	// a=<attribute>:<value>
	// https://tools.ietf.org/html/rfc4566#section-5.13
	Attributes []Attribute

	// https://tools.ietf.org/html/rfc4566#section-5.14
	MediaDescriptions []MediaDescription
}

type ProtocolVersion int

func (v *ProtocolVersion) String() *string {
	output := strconv.Itoa(int(*v))
	return &output
}

type Origin struct {
	Username       string
	SessionId      uint64
	SessionVersion uint64
	NetworkType    string
	AddressType    string
	UnicastAddress string
}

func (o *Origin) String() *string {
	output := fmt.Sprintf(
		"%v %d %d %v %v %v",
		o.Username,
		o.SessionId,
		o.SessionVersion,
		o.NetworkType,
		o.AddressType,
		o.UnicastAddress,
	)
	return &output
}

type SessionName string

func (s *SessionName) String() *string {
	output := string(*s)
	return &output
}

type URI url.URL

func (u *URI) String() *string {
	output := url.URL(*u).String()
	return &output
}

type EmailAddress string

func (e *EmailAddress) String() *string {
	output := string(*e)
	return &output
}

type PhoneNumber string

func (p *PhoneNumber) String() *string {
	output := string(*p)
	return &output
}

type TimeZone struct {
	AdjustmentTime uint64
	Offset         int64
}

func (z *TimeZone) String() string {
	return strconv.FormatUint(z.AdjustmentTime, 10) + " " + strconv.FormatInt(z.Offset, 10)
}
