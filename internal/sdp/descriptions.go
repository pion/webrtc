package sdp

import (
	"fmt"
	"net/url"
	"net"
	"strings"
	"strconv"
)

type Origin struct {
	Username       string
	SessionId      uint64
	SessionVersion uint64
	NetworkType    string
	AddressType    string
	UnicastAddress string
}

func (o *Origin) String() string {
	return fmt.Sprintf(
		"%v %d %d %v %v %v",
		o.Username,
		o.SessionId,
		o.SessionVersion,
		o.NetworkType,
		o.AddressType,
		o.UnicastAddress,
	)
}

type ConnectionAddress struct {
	Address net.IP
	Ttl     *int
	Multi   *int
}

func (c *ConnectionAddress) String() string {
	var parts []string
	parts = append(parts, c.Address.String())
	if c.Ttl != nil {
		parts = append(parts, strconv.Itoa(*c.Ttl))
	}

	if c.Multi != nil {
		parts = append(parts, strconv.Itoa(*c.Multi))
	}

	return strings.Join(parts, "/")
}

type ConnectionInformation struct {
	NetworkType       string
	AddressType       string
	ConnectionAddress *ConnectionAddress
}

func (c *ConnectionInformation) String() string {
	return fmt.Sprintf(
		"%v %v %v",
		c.NetworkType,
		c.AddressType,
		c.ConnectionAddress.String(),
	)
}

// SessionDescription is a a well-defined format for conveying sufficient
// information to discover and participate in a multimedia session.
type SessionDescription struct {
	// ProtocolVersion gives the version of the Session Description Protocol
	// https://tools.ietf.org/html/rfc4566#section-5.1
	ProtocolVersion int

	// Origin gives the originator of the session in the form of
	// o=<username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
	// https://tools.ietf.org/html/rfc4566#section-5.2
	Origin Origin

	// SessionName is the textual session name. There MUST be one and only one
	// only one "s=" field per session description
	// https://tools.ietf.org/html/rfc4566#section-5.3
	SessionName string

	// SessionInformation field provides textual information about the session.  There
	// MUST be at most one session-level SessionInformation field per session description,
	// and at most one SessionInformation field per media
	// https://tools.ietf.org/html/rfc4566#section-5.4
	SessionInformation *string

	// URI is a pointer to additional information about the
	// session.  This field is OPTIONAL, but if it is present it MUST be
	// specified before the first media field.  No more than one URI field
	// is allowed per session description.
	// https://tools.ietf.org/html/rfc4566#section-5.5
	URI *url.URL

	// EmailAddress specifies the email for the person responsible for the conference
	// https://tools.ietf.org/html/rfc4566#section-5.6
	EmailAddress *string

	// PhoneNumber specifies the phone number for the person responsible for the conference
	// https://tools.ietf.org/html/rfc4566#section-5.6
	PhoneNumber *string

	// ConnectionInformation a session description MUST contain either at least one ConnectionInformation field in
	// each media description or a single ConnectionInformation field at the session level.
	// https://tools.ietf.org/html/rfc4566#section-5.7
	ConnectionInformation *ConnectionInformation

	// Bandwidth field denotes the proposed bandwidth to be used by the
	// session or media
	// b=<bwtype>:<bandwidth>
	// https://tools.ietf.org/html/rfc4566#section-5.8
	Bandwidth []string

	// Timing lines specify the start and stop times for a session.
	// t=<start-time> <stop-time>
	// https://tools.ietf.org/html/rfc4566#section-5.9
	Timing []string

	// RepeatTimes specify repeat times for a session
	// r=<repeat interval> <active duration> <offsets from start-time>
	// https://tools.ietf.org/html/rfc4566#section-5.10
	RepeatTimes []string

	// TimeZones schedule a repeated session that spans a change from daylight
	// z=<adjustment time> <offset> <adjustment time> <offset>
	// https://tools.ietf.org/html/rfc4566#section-5.11
	TimeZones []string

	// EncryptionKeys if for when the SessionDescription is transported over a secure and trusted channel,
	// the Session Description Protocol MAY be used to convey encryption keys
	// https://tools.ietf.org/html/rfc4566#section-5.11
	EncryptionKeys []string

	// Attributes are the primary means for extending SDP.  Attributes may
	// be defined to be used as "session-level" attributes, "media-level"
	// attributes, or both.
	// https://tools.ietf.org/html/rfc4566#section-5.12
	Attributes []string

	// MediaDescriptions A session description may contain a number of media descriptions.
	// Each media description starts with an "m=" field and is terminated by
	// either the next "m=" field or by the end of the session description.
	// https://tools.ietf.org/html/rfc4566#section-5.13
	MediaDescriptions []*MediaDescription
}

// MediaDescription represents a media type.  Currently defined media are "audio",
// "video", "text", "application", and "message", although this list
// may be extended in the future
// https://tools.ietf.org/html/rfc4566#section-5.14
type MediaDescription struct {
	// MediaName is m=<media> <port> <proto> <fmt>
	// <media> is the media type
	// <port> is the transport port to which the media stream is sent
	// <proto> is the transport protocol
	// <fmt> is a media format description
	// https://tools.ietf.org/html/rfc4566#section-5.13
	MediaName string

	// SessionInformation field provides textual information about the session.  There
	// MUST be at most one session-level SessionInformation field per session description,
	// and at most one SessionInformation field per media
	// https://tools.ietf.org/html/rfc4566#section-5.4
	MediaInformation string

	// ConnectionInformation a session description MUST contain either at least one ConnectionInformation field in
	// each media description or a single ConnectionInformation field at the session level.
	// https://tools.ietf.org/html/rfc4566#section-5.7
	ConnectionData string

	// Bandwidth field denotes the proposed bandwidth to be used by the
	// session or media
	// b=<bwtype>:<bandwidth>
	// https://tools.ietf.org/html/rfc4566#section-5.8
	Bandwidth []string

	// EncryptionKeys if for when the SessionDescription is transported over a secure and trusted channel,
	// the Session Description Protocol MAY be used to convey encryption keys
	// https://tools.ietf.org/html/rfc4566#section-5.11
	EncryptionKeys []string

	// Attributes are the primary means for extending SDP.  Attributes may
	// be defined to be used as "session-level" attributes, "media-level"
	// attributes, or both.
	// https://tools.ietf.org/html/rfc4566#section-5.12
	Attributes []string
}
