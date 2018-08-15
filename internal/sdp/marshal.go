package sdp

import (
	"strings"
)

// Marshal takes a SDP struct to text
// https://tools.ietf.org/html/rfc4566#section-5
// Session description
//    v=  (protocol version)
//    o=  (originator and session identifier)
//    s=  (session name)
//    i=* (session information)
//    u=* (URI of description)
//    e=* (email address)
//    p=* (phone number)
//    c=* (connection information -- not required if included in
//         all media)
//    b=* (zero or more bandwidth information lines)
//    One or more time descriptions ("t=" and "r=" lines; see below)
//    z=* (time zone adjustments)
//    k=* (encryption key)
//    a=* (zero or more session attribute lines)
//    Zero or more media descriptions
//
// Time description
//    t=  (time the session is active)
//    r=* (zero or more repeat times)
//
// Media description, if present
//    m=  (media name and transport address)
//    i=* (media title)
//    c=* (connection information -- optional if included at
//         session level)
//    b=* (zero or more bandwidth information lines)
//    k=* (encryption key)
//    a=* (zero or more media attribute lines)
func (s *SessionDescription) Marshal() (raw string) {
	raw += keyValueBuild("v=", s.Version.String())
	raw += keyValueBuild("o=", s.Origin.String())
	raw += keyValueBuild("s=", s.SessionName.String())

	if s.SessionInformation != nil {
		raw += keyValueBuild("i=", s.SessionInformation.String())
	}

	if s.URI != nil {
		uri := s.URI.String()
		raw += keyValueBuild("u=", &uri)
	}

	if s.EmailAddress != nil {
		raw += keyValueBuild("e=", s.EmailAddress.String())
	}

	if s.PhoneNumber != nil {
		raw += keyValueBuild("p=", s.PhoneNumber.String())
	}

	if s.ConnectionInformation != nil {
		raw += keyValueBuild("c=", s.ConnectionInformation.String())
	}

	for _, b := range s.Bandwidth {
		raw += keyValueBuild("b=", b.String())
	}

	for _, td := range s.TimeDescriptions {
		raw += keyValueBuild("t=", td.Timing.String())
		for _, r := range td.RepeatTimes {
			raw += keyValueBuild("r=", r.String())
		}
	}

	rawTimeZones := make([]string, 0)
	for _, z := range s.TimeZones {
		rawTimeZones = append(rawTimeZones, z.String())
	}

	if len(rawTimeZones) > 0 {
		timeZones := strings.Join(rawTimeZones, " ")
		raw += keyValueBuild("z=", &timeZones)
	}

	if s.EncryptionKey != nil {
		raw += keyValueBuild("k=", s.EncryptionKey.String())
	}

	for _, a := range s.Attributes {
		raw += keyValueBuild("a=", a.String())
	}

	for _, md := range s.MediaDescriptions {
		raw += keyValueBuild("m=", md.MediaName.String())

		if md.MediaTitle != nil {
			raw += keyValueBuild("i=", md.MediaTitle.String())
		}

		if md.ConnectionInformation != nil {
			raw += keyValueBuild("c=", md.ConnectionInformation.String())
		}

		for _, b := range md.Bandwidth {
			raw += keyValueBuild("b=", b.String())
		}

		if md.EncryptionKey != nil {
			raw += keyValueBuild("k=", md.EncryptionKey.String())
		}

		for _, a := range md.Attributes {
			raw += keyValueBuild("a=", a.String())
		}
	}

	return raw
}
