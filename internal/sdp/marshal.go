package sdp

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func nextLine(scanner *bufio.Scanner) (key, value string, scanStatus bool, err error) {
	if scanStatus = scanner.Scan(); !scanStatus {
		return key, value, scanStatus, scanner.Err()
	}

	if len(scanner.Text()) < 3 {
		return key, value, scanStatus, errors.Errorf("line is not long enough to contain both a key and value: %s", scanner.Text())
	} else if scanner.Text()[1] != '=' {
		return key, value, scanStatus, errors.Errorf("line is not a proper key value pair, second character is not `=`: %s", scanner.Text())
	}

	return string(scanner.Text()[0]), scanner.Text()[2:], scanStatus, err
}

// Marshaw populates a SessionDescription with a raw string
//
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
// b=* (zero or more bandwidth information lines) One or more time descriptions ("t=" and "r=" lines; see below)
// z=* (time zone adjustments)
// k=* (encryption key)
// a=* (zero or more session attribute lines)
// Zero or more media descriptions
// https://tools.ietf.org/html/rfc4566#section-5
func (s *SessionDescription) Marshal(raw string) error {
	earlyEndErr := errors.Errorf("session description ended before all required values were found")

	s.Reset()
	scanner := bufio.NewScanner(strings.NewReader(raw))

	// v=
	key, value, scanStatus, err := nextLine(scanner)
	if err != nil {
		return err
	} else if !scanStatus {
		return earlyEndErr
	} else if key != "v" {
		return errors.Errorf("v (protocol version) was expected, but not found")
	} else if s.ProtocolVersion, err = strconv.Atoi(value); err != nil {
		return errors.Errorf("Failed to take protocol version to int")
	}

	// o=
	key, value, scanStatus, err = nextLine(scanner)
	if err != nil {
		return err
	} else if !scanStatus {
		return earlyEndErr
	} else if key != "o" {
		return errors.Errorf("o (originator and session identifier) was expected, but not found")
	}
	s.Origin = value

	return nil
}
