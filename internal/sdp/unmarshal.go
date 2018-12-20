package sdp

import (
	"bufio"
	"strings"

	"io"
	"net"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
)

// Unmarshal is the primary function that deserializes the session description
// message and stores it inside of a structured SessionDescription object.
//
// The States Sransition Table describes the computation flow between functions
// (namely s1, s2, s3, ...) for a parsing procedure that complies with the
// specifications laid out by the rfc4566#section-5 as well as by JavaScript
// Session Establishment Protocol draft. Links:
// 		https://tools.ietf.org/html/rfc4566#section-5
// 		https://tools.ietf.org/html/draft-ietf-rtcweb-jsep-24
//
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
//
// In order to generate the following state table and draw subsequent
// deterministic finite-state automota ("DFA") the following regex was used to
// derive the DFA:
// 		vosi?u?e?p?c?b*(tr*)+z?k?a*(mi?c?b*k?a*)*
//
// Please pay close attention to the `k`, and `a` parsing states. In the table
// below in order to distinguish between the states belonging to the media
// description as opposed to the session description, the states are marked
// with an asterisk ("a*", "k*").
// +--------+----+-------+----+-----+----+-----+---+----+----+---+---+-----+---+---+----+---+----+
// | STATES | a* | a*,k* | a  | a,k | b  | b,c | e | i  | m  | o | p | r,t | s | t | u  | v | z  |
// +--------+----+-------+----+-----+----+-----+---+----+----+---+---+-----+---+---+----+---+----+
// |   s1   |    |       |    |     |    |     |   |    |    |   |   |     |   |   |    | 2 |    |
// |   s2   |    |       |    |     |    |     |   |    |    | 3 |   |     |   |   |    |   |    |
// |   s3   |    |       |    |     |    |     |   |    |    |   |   |     | 4 |   |    |   |    |
// |   s4   |    |       |    |     |    |   5 | 6 |  7 |    |   | 8 |     |   | 9 | 10 |   |    |
// |   s5   |    |       |    |     |  5 |     |   |    |    |   |   |     |   | 9 |    |   |    |
// |   s6   |    |       |    |     |    |   5 |   |    |    |   | 8 |     |   | 9 |    |   |    |
// |   s7   |    |       |    |     |    |   5 | 6 |    |    |   | 8 |     |   | 9 | 10 |   |    |
// |   s8   |    |       |    |     |    |   5 |   |    |    |   |   |     |   | 9 |    |   |    |
// |   s9   |    |       |    |  11 |    |     |   |    | 12 |   |   |   9 |   |   |    |   | 13 |
// |   s10  |    |       |    |     |    |   5 | 6 |    |    |   | 8 |     |   | 9 |    |   |    |
// |   s11  |    |       | 11 |     |    |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |   s12  |    |    14 |    |     |    |  15 |   | 16 | 12 |   |   |     |   |   |    |   |    |
// |   s13  |    |       |    |  11 |    |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |   s14  | 14 |       |    |     |    |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |   s15  |    |    14 |    |     | 15 |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |   s16  |    |    14 |    |     |    |  15 |   |    | 12 |   |   |     |   |   |    |   |    |
// +--------+----+-------+----+-----+----+-----+---+----+----+---+---+-----+---+---+----+---+----+
func (s *SessionDescription) Unmarshal(value string) error {
	l := &lexer{
		desc:  s,
		input: bufio.NewReader(strings.NewReader(value)),
	}
	for state := s1; state != nil; {
		var err error
		state, err = state(l)
		if err != nil {
			return err
		}
	}
	return nil

}

func s1(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	if key == "v=" {
		return unmarshalProtocolVersion, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s2(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	if key == "o=" {
		return unmarshalOrigin, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s3(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, err
	}

	if key == "s=" {
		return unmarshalSessionName, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s4(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	switch key {
	case "i=":
		return unmarshalSessionInformation, nil
	case "u=":
		return unmarshalURI, nil
	case "e=":
		return unmarshalEmail, nil
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionInformation, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s5(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, err
	}

	switch key {
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s6(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	switch key {
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionInformation, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s7(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	switch key {
	case "u=":
		return unmarshalURI, nil
	case "e=":
		return unmarshalEmail, nil
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionInformation, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s8(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	switch key {
	case "c=":
		return unmarshalSessionConnectionInformation, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s9(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "z=":
		return unmarshalTimeZones, nil
	case "k=":
		return unmarshalSessionEncryptionKey, nil
	case "a=":
		return unmarshalSessionAttribute, nil
	case "r=":
		return unmarshalRepeatTimes, nil
	case "t=":
		return unmarshalTiming, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s10(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
	}

	switch key {
	case "e=":
		return unmarshalEmail, nil
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionInformation, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s11(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "a=":
		return unmarshalSessionAttribute, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s12(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "a=":
		return unmarshalMediaAttribute, nil
	case "k=":
		return unmarshalMediaEncryptionKey, nil
	case "b=":
		return unmarshalMediaBandwidth, nil
	case "c=":
		return unmarshalMediaConnectionInformation, nil
	case "i=":
		return unmarshalMediaTitle, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s13(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "a=":
		return unmarshalSessionAttribute, nil
	case "k=":
		return unmarshalSessionEncryptionKey, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s14(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "a=":
		return unmarshalMediaAttribute, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s15(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "a=":
		return unmarshalMediaAttribute, nil
	case "k=":
		return unmarshalMediaEncryptionKey, nil
	case "b=":
		return unmarshalMediaBandwidth, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func s16(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		if err == io.EOF && key == "" {
			return nil, nil
		}
		return nil, err
	}

	switch key {
	case "a=":
		return unmarshalMediaAttribute, nil
	case "k=":
		return unmarshalMediaEncryptionKey, nil
	case "c=":
		return unmarshalMediaConnectionInformation, nil
	case "b=":
		return unmarshalMediaBandwidth, nil
	case "m=":
		return unmarshalMediaDescription, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func unmarshalProtocolVersion(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	version, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid numeric value `%v`", version)
	}

	// As off the latest draft of the rfc this value is required to be 0.
	// https://tools.ietf.org/html/draft-ietf-rtcweb-jsep-24#section-5.8.1
	if version != 0 {
		return nil, errors.Errorf("sdp: invalid value `%v`", version)
	}

	return s2, nil
}

func unmarshalOrigin(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(value)
	if len(fields) != 6 {
		return nil, errors.Errorf("sdp: invalid syntax `o=%v`", fields)
	}

	sessionID, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid numeric value `%v`", fields[1])
	}

	sessionVersion, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid numeric value `%v`", fields[2])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.6
	if i := indexOf(fields[3], []string{"IN"}); i == -1 {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[3])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.7
	if i := indexOf(fields[4], []string{"IP4", "IP6"}); i == -1 {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[4])
	}

	// TODO validated UnicastAddress

	l.desc.Origin = Origin{
		Username:       fields[0],
		SessionID:      sessionID,
		SessionVersion: sessionVersion,
		NetworkType:    fields[3],
		AddressType:    fields[4],
		UnicastAddress: fields[5],
	}

	return s3, nil
}

func unmarshalSessionName(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.SessionName = SessionName(value)
	return s4, nil
}

func unmarshalSessionInformation(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	sessionInformation := Information(value)
	l.desc.SessionInformation = &sessionInformation
	return s7, nil
}

func unmarshalURI(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.URI, err = url.Parse(value)
	if err != nil {
		return nil, err
	}

	return s10, nil
}

func unmarshalEmail(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	emailAddress := EmailAddress(value)
	l.desc.EmailAddress = &emailAddress
	return s6, nil
}

func unmarshalPhone(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	phoneNumber := PhoneNumber(value)
	l.desc.PhoneNumber = &phoneNumber
	return s8, nil
}

func unmarshalSessionConnectionInformation(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.ConnectionInformation, err = unmarshalConnectionInformation(value)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `c=%v`", value)
	}
	return s5, nil
}

func unmarshalConnectionInformation(value string) (*ConnectionInformation, error) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil, errors.Errorf("sdp: invalid syntax `c=%v`", fields)
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.6
	if i := indexOf(fields[0], []string{"IN"}); i == -1 {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[0])
	}

	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-8.2.7
	if i := indexOf(fields[1], []string{"IP4", "IP6"}); i == -1 {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[1])
	}

	var connAddr *Address
	if len(fields) > 2 {
		connAddr = &Address{}

		parts := strings.Split(fields[2], "/")
		connAddr.IP = net.ParseIP(parts[0])
		if connAddr.IP == nil {
			return nil, errors.Errorf("sdp: invalid value `%v`", fields[2])
		}

		isIP6 := connAddr.IP.To4() == nil
		if len(parts) > 1 {
			val, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				return nil, errors.Errorf("sdp: invalid numeric value `%v`", fields[2])
			}

			if isIP6 {
				multi := int(val)
				connAddr.Range = &multi
			} else {
				ttl := int(val)
				connAddr.TTL = &ttl
			}
		}

		if len(parts) > 2 {
			val, err := strconv.ParseInt(parts[2], 10, 32)
			if err != nil {
				return nil, errors.Errorf("sdp: invalid numeric value `%v`", fields[2])
			}

			multi := int(val)
			connAddr.Range = &multi
		}

	}

	return &ConnectionInformation{
		NetworkType: fields[0],
		AddressType: fields[1],
		Address:     connAddr,
	}, nil
}

func unmarshalSessionBandwidth(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	bandwidth, err := unmarshalBandwidth(value)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `b=%v`", value)
	}
	l.desc.Bandwidth = append(l.desc.Bandwidth, *bandwidth)

	return s5, nil
}

func unmarshalBandwidth(value string) (*Bandwidth, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return nil, errors.Errorf("sdp: invalid syntax `b=%v`", parts)
	}

	experimental := strings.HasPrefix(parts[0], "X-")
	if experimental {
		parts[0] = strings.TrimPrefix(parts[0], "X-")
	} else {
		// Set according to currently registered with IANA
		// https://tools.ietf.org/html/rfc4566#section-5.8
		if i := indexOf(parts[0], []string{"CT", "AS"}); i == -1 {
			return nil, errors.Errorf("sdp: invalid value `%v`", parts[0])
		}
	}

	bandwidth, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid numeric value `%v`", parts[1])
	}

	return &Bandwidth{
		Experimental: experimental,
		Type:         parts[0],
		Bandwidth:    bandwidth,
	}, nil
}

func unmarshalTiming(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(value)
	if len(fields) < 2 {
		return nil, errors.Errorf("sdp: invalid syntax `t=%v`", fields)
	}

	td := TimeDescription{}

	td.Timing.StartTime, err = strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid numeric value `%v`", fields[1])
	}

	td.Timing.StopTime, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid numeric value `%v`", fields[1])
	}

	l.desc.TimeDescriptions = append(l.desc.TimeDescriptions, td)

	return s9, nil
}

func unmarshalRepeatTimes(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(value)
	if len(fields) < 3 {
		return nil, errors.Errorf("sdp: invalid syntax `r=%v`", fields)
	}

	latestTimeDesc := &l.desc.TimeDescriptions[len(l.desc.TimeDescriptions)-1]

	newRepeatTime := RepeatTime{}
	newRepeatTime.Interval, err = parseTimeUnits(fields[0])
	if err != nil {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields)
	}

	newRepeatTime.Duration, err = parseTimeUnits(fields[1])
	if err != nil {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields)
	}

	for i := 2; i < len(fields); i++ {
		offset, err := parseTimeUnits(fields[i])
		if err != nil {
			return nil, errors.Errorf("sdp: invalid value `%v`", fields)
		}
		newRepeatTime.Offsets = append(newRepeatTime.Offsets, offset)
	}
	latestTimeDesc.RepeatTimes = append(latestTimeDesc.RepeatTimes, newRepeatTime)

	return s9, nil
}

func unmarshalTimeZones(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	// These fields are transimitted in pairs
	// z=<adjustment time> <offset> <adjustment time> <offset> ....
	// so we are making sure that there are actually multiple of 2 total.
	fields := strings.Fields(value)
	if len(fields)%2 != 0 {
		return nil, errors.Errorf("sdp: invalid syntax `t=%v`", fields)
	}

	for i := 0; i < len(fields); i += 2 {
		var timeZone TimeZone

		timeZone.AdjustmentTime, err = strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return nil, errors.Errorf("sdp: invalid value `%v`", fields)
		}

		timeZone.Offset, err = parseTimeUnits(fields[i+1])
		if err != nil {
			return nil, err
		}

		l.desc.TimeZones = append(l.desc.TimeZones, timeZone)
	}

	return s13, nil
}

func unmarshalSessionEncryptionKey(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	encryptionKey := EncryptionKey(value)
	l.desc.EncryptionKey = &encryptionKey
	return s11, nil
}

func unmarshalSessionAttribute(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	i := strings.IndexRune(value, ':')
	var a Attribute
	if i > 0 {
		a = NewAttribute(value[:i], value[i+1:])
	} else {
		a = NewPropertyAttribute(value)
	}

	l.desc.Attributes = append(l.desc.Attributes, a)
	return s11, nil
}

func unmarshalMediaDescription(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(value)
	if len(fields) < 4 {
		return nil, errors.Errorf("sdp: invalid syntax `m=%v`", fields)
	}

	newMediaDesc := &MediaDescription{}

	// <media>
	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-5.14
	if i := indexOf(fields[0], []string{"audio", "video", "text", "application", "message"}); i == -1 {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[0])
	}
	newMediaDesc.MediaName.Media = fields[0]

	// <port>
	parts := strings.Split(fields[1], "/")
	newMediaDesc.MediaName.Port.Value, err = parsePort(parts[0])
	if err != nil {
		return nil, errors.Errorf("sdp: invalid port value `%v`", parts[0])
	}

	if len(parts) > 1 {
		portRange, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, errors.Errorf("sdp: invalid value `%v`", parts)
		}
		newMediaDesc.MediaName.Port.Range = &portRange
	}

	// <proto>
	// Set according to currently registered with IANA
	// https://tools.ietf.org/html/rfc4566#section-5.14
	for _, proto := range strings.Split(fields[2], "/") {
		if i := indexOf(proto, []string{"UDP", "RTP", "AVP", "SAVP", "SAVPF", "TLS", "DTLS", "SCTP"}); i == -1 {
			return nil, errors.Errorf("sdp: invalid value `%v`", fields[2])
		}
		newMediaDesc.MediaName.Protos = append(newMediaDesc.MediaName.Protos, proto)
	}

	// <fmt>...
	for i := 3; i < len(fields); i++ {
		newMediaDesc.MediaName.Formats = append(newMediaDesc.MediaName.Formats, fields[i])
	}

	l.desc.MediaDescriptions = append(l.desc.MediaDescriptions, newMediaDesc)

	return s12, nil
}

func unmarshalMediaTitle(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	latestMediaDesc := l.desc.MediaDescriptions[len(l.desc.MediaDescriptions)-1]
	mediaTitle := Information(value)
	latestMediaDesc.MediaTitle = &mediaTitle
	return s16, nil
}

func unmarshalMediaConnectionInformation(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	latestMediaDesc := l.desc.MediaDescriptions[len(l.desc.MediaDescriptions)-1]
	latestMediaDesc.ConnectionInformation, err = unmarshalConnectionInformation(value)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `c=%v`", value)
	}
	return s15, nil
}

func unmarshalMediaBandwidth(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	latestMediaDesc := l.desc.MediaDescriptions[len(l.desc.MediaDescriptions)-1]
	bandwidth, err := unmarshalBandwidth(value)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid syntax `b=%v`", value)
	}
	latestMediaDesc.Bandwidth = append(latestMediaDesc.Bandwidth, *bandwidth)
	return s15, nil
}

func unmarshalMediaEncryptionKey(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	latestMediaDesc := l.desc.MediaDescriptions[len(l.desc.MediaDescriptions)-1]
	encryptionKey := EncryptionKey(value)
	latestMediaDesc.EncryptionKey = &encryptionKey
	return s14, nil
}

func unmarshalMediaAttribute(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	i := strings.IndexRune(value, ':')
	var a Attribute
	if i > 0 {
		a = NewAttribute(value[:i], value[i+1:])
	} else {
		a = NewPropertyAttribute(value)
	}

	latestMediaDesc := l.desc.MediaDescriptions[len(l.desc.MediaDescriptions)-1]
	latestMediaDesc.Attributes = append(latestMediaDesc.Attributes, a)
	return s14, nil
}

func parseTimeUnits(value string) (int64, error) {
	// Some time offsets in the protocol can be provided with a shorthand
	// notation. This code ensures to convert it to NTP timestamp format.
	//      d - days (86400 seconds)
	//      h - hours (3600 seconds)
	//      m - minutes (60 seconds)
	//      s - seconds (allowed for completeness)
	switch value[len(value)-1:] {
	case "d":
		num, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		if err != nil {
			return 0, errors.Errorf("sdp: invalid value `%v`", value)
		}
		return num * 86400, nil
	case "h":
		num, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		if err != nil {
			return 0, errors.Errorf("sdp: invalid value `%v`", value)
		}
		return num * 3600, nil
	case "m":
		num, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		if err != nil {
			return 0, errors.Errorf("sdp: invalid value `%v`", value)
		}
		return num * 60, nil
	}

	num, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.Errorf("sdp: invalid value `%v`", value)
	}

	return num, nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.Errorf("sdp: invalid port value `%v`", port)
	}

	if port < 0 || port > 65536 {
		return 0, errors.Errorf("sdp: invalid port value -- out of range `%v`", port)
	}

	return port, nil
}
