package sdp

import "strconv"

func kvBuilder(key, value string) string {
	return key + "=" + value + "\n"
}

// Marshal creates a raw string from a SessionDescription
// Some lines in each description are REQUIRED and some are OPTIONAL,
// but all MUST appear in exactly the order given here (the fixed order
// greatly enhances error detection and allows for a simple parser).
// OPTIONAL items are marked with a "*".
// v=  (protocol version)
// o=  (originator and session identifier)
// s=  (session name)
// i=* (session information)
// u=* (URI of description)
// e=* (email address)
// p=* (phone number)
// c=* (connection information -- not required if included in all media)
// b=* (zero or more bandwidth information lines)
// t=* (One or more time descriptions)
// r=* (One or more repeat descriptions)
// z=* (time zone adjustments)
// k=* (encryption key)
// a=* (zero or more session attribute lines)
// Zero or more media descriptions
// https://tools.ietf.org/html/rfc4566#section-5
func (s *SessionDescription) Marshal() (raw string) {
	addIfSet := func(key, value string) {
		if value != "" {
			raw += kvBuilder(key, value)
		}
	}
	addSlice := func(key string, values []string) {
		for _, v := range values {
			raw += kvBuilder(key, v)
		}
	}

	raw += kvBuilder("v", strconv.Itoa(s.ProtocolVersion))
	raw += kvBuilder("o", s.Origin.String())
	raw += kvBuilder("s", s.SessionName)

	if s.SessionInformation != nil {
		raw += kvBuilder("i", *s.SessionInformation)
	}

	if s.URI != nil {
		raw += kvBuilder("u", s.URI.String())
	}

	if s.EmailAddress != nil {
		raw += kvBuilder("e", *s.EmailAddress)
	}

	if s.PhoneNumber != nil {
		raw += kvBuilder("p", *s.PhoneNumber)
	}

	if s.ConnectionInformation != nil {
		raw += kvBuilder("c", s.ConnectionInformation.String())
	}

	addSlice("b", s.Bandwidth)
	addSlice("t", s.Timing)
	addSlice("r", s.RepeatTimes)
	addSlice("z", s.TimeZones)
	addSlice("k", s.EncryptionKeys)
	addSlice("a", s.Attributes)

	for _, a := range s.MediaDescriptions {
		raw += kvBuilder("m", a.MediaName)

		addIfSet("i", a.MediaInformation)
		addIfSet("c", a.ConnectionData)

		addSlice("b", a.Bandwidth)
		addSlice("k", a.EncryptionKeys)
		addSlice("a", a.Attributes)
	}

	return raw
}
