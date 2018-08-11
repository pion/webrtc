package sdp

import (
	"bufio"
	"strings"

	"github.com/pkg/errors"
	"strconv"
	"io"
	"net/url"
		"net"
)

// States transition table
// +--------+----+-----+----+-----+---+----+----+---+---+-----+---+---+----+---+----+
// | STATES | a  | a,k | b  | b,c | e | i  | m  | o | p | r,t | s | t | u  | v | z  |
// +--------+----+-----+----+-----+---+----+----+---+---+-----+---+---+----+---+----+
// |  s1    |    |     |    |     |   |    |    |   |   |     |   |   |    | 2 |    |
// |  s2    |    |     |    |     |   |    |    | 3 |   |     |   |   |    |   |    |
// |  s3    |    |     |    |     |   |    |    |   |   |     | 4 |   |    |   |    |
// |  s4    |    |     |    |   5 | 6 |  7 |    |   | 8 |     |   | 9 | 10 |   |    |
// |  s5    |    |     |  5 |     |   |    |    |   |   |     |   | 9 |    |   |    |
// |  s6    |    |     |    |   5 |   |    |    |   | 8 |     |   | 9 |    |   |    |
// |  s7    |    |     |    |   5 | 6 |    |    |   | 8 |     |   | 9 | 10 |   |    |
// |  s8    |    |     |    |   5 |   |    |    |   |   |     |   | 9 |    |   |    |
// |  s9    |    |  11 |    |     |   |    | 12 |   |   |   9 |   |   |    |   | 13 |
// |  s10   |    |     |    |   5 | 6 |    |    |   | 8 |     |   | 9 |    |   |    |
// |  s11   | 11 |     |    |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |  s12   |    |  11 |    |  14 |   | 15 | 12 |   |   |     |   |   |    |   |    |
// |  s13   |    |  11 |    |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |  s14   |    |  11 | 14 |     |   |    | 12 |   |   |     |   |   |    |   |    |
// |  s15   |    |  11 |    |  14 |   |    | 12 |   |   |     |   |   |    |   |    |
// +--------+----+-----+----+-----+---+----+----+---+---+-----+---+---+----+---+----+

var (
	ErrSyntax = errors.New("sdp: invalid syntax")
)

type lexer struct {
	desc  *SessionDescription
	input *bufio.Reader
}

func readType(input *bufio.Reader) (string, error) {
	key, err := input.ReadString('=')
	if err != nil {
		return key, err
	}

	if len(key) != 2 {
		return key, ErrSyntax
	}

	return key, nil
}

func readValue(input *bufio.Reader) (string, error) {
	line, err := input.ReadString('\n')
	if err != nil && err != io.EOF {
		return line, err
	}

	if len(line) == 0 {
		return line, nil
	}

	if line[len(line)-1] == '\n' {
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			drop = 2
		}
		line = line[:len(line)-drop]
	}

	return line, nil
}

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1
}

type stateFn func(*lexer) (stateFn, error)

func (s *SessionDescription) Unmarshal(value string) error {
	l := &lexer{s, bufio.NewReader(strings.NewReader(value))}
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
		return nil, err
	}

	if key == "v=" {
		return unmarshalProtocolVersion, nil
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
		return nil, errors.Errorf("sdp: invalid syntax `v=%v`", version)
	}

	// As off the latest draft of the rfc this value is required to be 0.
	// https://tools.ietf.org/html/draft-ietf-rtcweb-jsep-24#section-5.8.1
	if version != 0 {
		return nil, errors.Errorf("sdp: invalid value `%v`", version)
	}

	return s2, nil
}

func s2(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, err
	}

	if key == "o=" {
		return unmarshalOrigin, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
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

	sessionId, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[1])
	}

	sessionVersion, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return nil, errors.Errorf("sdp: invalid value `%v`", fields[2])
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
		Username: fields[0],
		SessionId: sessionId,
		SessionVersion: sessionVersion,
		NetworkType: fields[3],
		AddressType: fields[4],
		UnicastAddress: fields[5],
	}

	return s3, nil
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
func unmarshalSessionName(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.SessionName = value
	return s4, nil
}

func s4(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, err
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
		return unmarshalSessionConnectionData, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

func unmarshalSessionInformation(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.SessionInformation = &value
	return s7, nil
}

func unmarshalURI(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	if l.desc.URI, err = url.Parse(value); err != nil {
		return nil, err
	}

	return s10, nil
}

func unmarshalEmail(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.EmailAddress = &value
	return s6, nil
}

func unmarshalPhone(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

	l.desc.PhoneNumber = &value
	return s8, nil
}

func unmarshalSessionConnectionData(l *lexer) (stateFn, error) {
	value, err := readValue(l.input)
	if err != nil {
		return nil, err
	}

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

	var connAddr *ConnectionAddress
	if len(fields) > 2 {
		connAddr = &ConnectionAddress{}

		parts := strings.Split(fields[2], "/")
		connAddr.Address = net.ParseIP(parts[0])
		if connAddr.Address == nil {
			return nil, errors.Errorf("sdp: invalid value `%v`", fields[2])
		}

		isIP6 := connAddr.Address.To4() == nil
		if len(parts) > 1 {
			val, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil {
				return nil, errors.Errorf("sdp: invalid value `%v`", fields[2])
			}

			if isIP6 {
				multi := int(val)
				connAddr.Multi = &multi
			} else {
				ttl := int(val)
				connAddr.Ttl = &ttl
			}
		}

		if len(parts) > 2 {
			val, err := strconv.ParseInt(parts[2], 10, 32)
			if err != nil {
				return nil, errors.Errorf("sdp: invalid value `%v`", fields[2])
			}

			multi := int(val)
			connAddr.Multi = &multi
		}

	}

	l.desc.ConnectionInformation = &ConnectionInformation{
		NetworkType: fields[0],
		AddressType: fields[1],
		ConnectionAddress: connAddr,
	}

	return s5, nil
}

func unmarshalSessionBandwidth(l *lexer) (stateFn, error) {
	// return s5, nil
	return nil, nil
}

func unmarshalTiming(l *lexer) (stateFn, error) {
	// return s9, nil
	return nil, nil
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
		return nil, err
	}

	switch key {
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionData, nil
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
		return nil, err
	}

	switch key {
	case "u=":
		return unmarshalURI, nil
	case "e=":
		return unmarshalEmail, nil
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionData, nil
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
		return nil, err
	}

	switch key {
	case "c=":
		return unmarshalSessionConnectionData, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

// func s9(l *lexer) (stateFn, error) {
// 	key, value, scanStatus, err := nextLine(scanner)
// 	if !scanStatus {
// 		return nil, err
// 	}
//
// 	switch key {
// 	case "z":
// 		if err := unmarshalTimeZones(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s13, nil
// 	case "k":
// 		if err := unmarshalSessionEncryptionKeys(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "a":
// 		if err := unmarshalSessionAttribute(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "r":
// 		if err := unmarshalRepeatTimes(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s9, nil
// 	case "t":
// 		if err := unmarshalTiming(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s9, nil
// 	case "m":
// 		if err := unmarshalMediaDescription(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s12, nil
// 	}
//
// 	return nil, errors.Errorf("")
// }

func s10(l *lexer) (stateFn, error) {
	key, err := readType(l.input)
	if err != nil {
		return nil, err
	}

	switch key {
	case "e=":
		return unmarshalEmail, nil
	case "p=":
		return unmarshalPhone, nil
	case "c=":
		return unmarshalSessionConnectionData, nil
	case "b=":
		return unmarshalSessionBandwidth, nil
	case "t=":
		return unmarshalTiming, nil
	}

	return nil, errors.Errorf("sdp: invalid syntax `%v`", key)
}

// func s11(l *lexer) (stateFn, error) {
// 	key, value, scanStatus, err := nextLine(scanner)
// 	if !scanStatus {
// 		return nil, err
// 	}
//
// 	switch key {
// 	case "a":
// 		if err := unmarshalMediaAttribute(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "m":
// 		if err := unmarshalMediaDescription(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s12, nil
// 	}
//
// 	return nil, errors.Errorf("")
// }
//
// func s12(l *lexer) (stateFn, error) {
// 	key, value, scanStatus, err := nextLine(scanner)
// 	if !scanStatus {
// 		return nil, err
// 	}
//
// 	switch key {
// 	case "a":
// 		if err := unmarshalAttribute(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "k":
// 		if err := unmarshalEncryptionKeys(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "b":
// 		if err := unmarshalBandwidth(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s14, nil
// 	case "c":
// 		if err := unmarshalMediaConnectionData(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s14, nil
// 	case "i":
// 		if err := unmarshalInformation(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s15, nil
// 	case "m":
// 		if err := unmarshalMediaDescription(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s12, nil
// 	}
//
// 	return nil, errors.Errorf("")
// }
//
// func s13(l *lexer) (stateFn, error) {
// 	key, value, scanStatus, err := nextLine(scanner)
// 	if !scanStatus {
// 		return nil, err
// 	}
//
// 	switch key {
// 	case "a":
// 		if err := unmarshalAttribute(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "k":
// 		if err := unmarshalEncryptionKeys(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "m":
// 		if err := unmarshalMediaDescription(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s12, nil
// 	}
//
// 	return nil, errors.Errorf("")
// }
//
// func s14(l *lexer) (stateFn, error) {
// 	key, value, scanStatus, err := nextLine(scanner)
// 	if !scanStatus {
// 		return nil, err
// 	}
//
// 	switch key {
// 	case "a":
// 		if err := unmarshalAttribute(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "k":
// 		if err := unmarshalEncryptionKeys(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "b":
// 		if err := unmarshalBandwidth(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s14, nil
// 	case "m":
// 		if err := unmarshalMediaDescription(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s12, nil
// 	}
//
// 	return nil, errors.Errorf("")
// }
//
// func s15(l *lexer) (stateFn, error) {
// 	key, value, scanStatus, err := nextLine(scanner)
// 	if !scanStatus {
// 		return nil, err
// 	}
//
// 	switch key {
// 	case "a":
// 		if err := unmarshalAttribute(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "k":
// 		if err := unmarshalEncryptionKeys(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s11, nil
// 	case "b":
// 		if err := unmarshalBandwidth(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s14, nil
// 	case "c":
// 		if err := unmarshalConnectionData(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s14, nil
// 	case "m":
// 		if err := unmarshalMediaDescription(value); err != nil {
// 			return nil, errors.Errorf("")
// 		}
// 		return s12, nil
// 	}
//
// 	return nil, errors.Errorf("")
// }
