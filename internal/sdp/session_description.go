package sdp

// SessionDescription is a a well-defined format for conveying sufficient
// information to discover and participate in a multimedia session.
type SessionDescription struct {
	// ProtocolVersion gives the version of the Session Description Protocol
	// https://tools.ietf.org/html/rfc4566#section-5.1
	ProtocolVersion int

	// Origin gives the originator of the session in the form of
	// o=<username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
	// https://tools.ietf.org/html/rfc4566#section-5.2
	Origin string

	// SessionName is the textual session name. There MUST be one and only one
	// only one "s=" field per session description
	// https://tools.ietf.org/html/rfc4566#section-5.3
	SessionName string

	// SessionInformation field provides textual information about the session.  There
	// MUST be at most one session-level SessionInformation field per session description,
	// and at most one SessionInformation field per media
	// https://tools.ietf.org/html/rfc4566#section-5.4
	SessionInformation string

	// URI is a pointer to additional information about the
	// session.  This field is OPTIONAL, but if it is present it MUST be
	// specified before the first media field.  No more than one URI field
	// is allowed per session description.
	// https://tools.ietf.org/html/rfc4566#section-5.5
	URI string

	// EmailAddress specifies the email for the person responsible for the conference
	// https://tools.ietf.org/html/rfc4566#section-5.6
	EmailAddress string

	// PhoneNumber specifies the phone number for the person responsible for the conference
	// https://tools.ietf.org/html/rfc4566#section-5.6
	PhoneNumber string

	// ConnectionData a session description MUST contain either at least one ConnectionData field in
	// each media description or a single ConnectionData field at the session level.
	// https://tools.ietf.org/html/rfc4566#section-5.7
	ConnectionData string

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

// Reset cleans the SessionDescription, and sets all fields back to their default values
func (s *SessionDescription) Reset() {
	s.ProtocolVersion = 0
	s.Origin = ""
	s.SessionName = ""
	s.SessionInformation = ""
	s.URI = ""
	s.EmailAddress = ""
	s.PhoneNumber = ""
	s.ConnectionData = ""
	s.Bandwidth = nil
	s.Timing = nil
	s.RepeatTimes = nil
	s.TimeZones = nil
	s.EncryptionKeys = nil
	s.Attributes = nil
	s.MediaDescriptions = nil
}
