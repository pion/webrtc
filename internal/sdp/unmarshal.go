package sdp

import (
	"bufio"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type attributeStatus struct {
	seen          bool
	value         string
	allowMultiple bool
}

// Detect if the current attribute is ok to be read (detect out of order errors)
// or if it has already been set
func attributeValid(statuses []*attributeStatus, attribute string) (err error) {
	attrFound := false
	for _, v := range statuses {
		if attrFound && v.seen {
			return errors.Errorf("Attribute %s was found, but later attribute %s has already been set", attribute, v.value)
		}

		if v.value == attribute {
			if v.seen && !v.allowMultiple {
				return errors.Errorf("Attribute %s was attempted to be set twice: %v", attribute, v.value)
			}
			attrFound = true
			v.seen = true
		}
	}
	return nil
}

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

// Unmarshal populates a SessionDescription from a raw string
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
// b=* (zero or more bandwidth information lines)
// t=* (One or more time descriptions)
// r=* (One or more repeat descriptions)
// z=* (time zone adjustments)
// k=* (encryption key)
// a=* (zero or more session attribute lines)
// Zero or more media descriptions
// https://tools.ietf.org/html/rfc4566#section-5
func (s *SessionDescription) Unmarshal(raw string) error {
	earlyEndErr := errors.Errorf("session description ended before all required values were found")
	orderedSessionAttributes := []*attributeStatus{
		&attributeStatus{value: "v"},
		&attributeStatus{value: "o"},
		&attributeStatus{value: "s"},
		&attributeStatus{value: "i"},
		&attributeStatus{value: "u"},
		&attributeStatus{value: "e"},
		&attributeStatus{value: "p"},
		&attributeStatus{value: "c"},
		&attributeStatus{value: "b", allowMultiple: true},
		&attributeStatus{value: "t", allowMultiple: true},
		&attributeStatus{value: "r", allowMultiple: true},
		&attributeStatus{value: "z", allowMultiple: true},
		&attributeStatus{value: "k", allowMultiple: true},
		&attributeStatus{value: "a", allowMultiple: true},
		&attributeStatus{value: "m", allowMultiple: true},
	}

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

	key, value, scanStatus, err = nextLine(scanner)
	if err != nil {
		return err
	} else if !scanStatus {
		return earlyEndErr
	} else if key != "s" {
		return errors.Errorf("o (session name) was expected, but not found")
	}
	s.SessionName = value

	for {
		key, value, scanStatus, err = nextLine(scanner)
		if err != nil || !scanStatus {
			return err
		}

		if err := attributeValid(orderedSessionAttributes, key); err != nil {
			return err
		}

		switch key {
		case "i":
			s.SessionInformation = value
		case "u":
			s.URI = value
		case "e":
			s.EmailAddress = value
		case "p":
			s.PhoneNumber = value
		case "c":
			s.ConnectionData = value
		case "b":
			s.Bandwidth = append(s.Bandwidth, value)
		case "t":
			s.Timing = append(s.Timing, value)
		case "r":
			s.RepeatTimes = append(s.RepeatTimes, value)
		case "z":
			s.TimeZones = append(s.TimeZones, value)
		case "k":
			s.EncryptionKeys = append(s.EncryptionKeys, value)
		case "a":
			s.Attributes = append(s.Attributes, value)
		case "m":
			return s.unmarshalMedias(scanner, value)
		default:
			return errors.Errorf("Invalid session attribute: %s", key)
		}
	}
}

func (s *SessionDescription) unmarshalMedias(scanner *bufio.Scanner, firstMediaName string) (err error) {
	currentMedia := &MediaDescription{MediaName: firstMediaName}
	orderedMediaAttributes := []*attributeStatus{
		&attributeStatus{value: "i"},
		&attributeStatus{value: "c"},
		&attributeStatus{value: "b", allowMultiple: true},
		&attributeStatus{value: "k", allowMultiple: true},
		&attributeStatus{value: "a", allowMultiple: true},
	}
	resetMediaAttributes := func() {
		for _, v := range orderedMediaAttributes {
			v.seen = false
		}
	}

	for {
		key, value, scanStatus, err := nextLine(scanner)
		if err != nil || !scanStatus { // This handles EOF, finish current MediaDescription
			s.MediaDescriptions = append(s.MediaDescriptions, currentMedia)
			return err
		}

		if err := attributeValid(orderedMediaAttributes, key); err != nil {
			return err
		}

		switch key {
		case "m":
			s.MediaDescriptions = append(s.MediaDescriptions, currentMedia)
			resetMediaAttributes()
			currentMedia = &MediaDescription{MediaName: value}
		case "i":
			currentMedia.MediaInformation = value
		case "c":
			currentMedia.ConnectionData = value
		case "b":
			currentMedia.Bandwidth = append(currentMedia.Bandwidth, value)
		case "k":
			currentMedia.EncryptionKeys = append(currentMedia.EncryptionKeys, value)
		case "a":
			currentMedia.Attributes = append(currentMedia.Attributes, value)
		default:
			return errors.Errorf("Invalid media attribute: %s", key)
		}
	}
}
