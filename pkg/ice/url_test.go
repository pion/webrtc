package ice

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseURL(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			rawURL            string
			expectedURLString string
			expectedScheme    SchemeType
			expectedSecure    bool
			expectedHost      string
			expectedPort      int
			expectedProto     ProtoType
		}{
			{"stun:google.de", "stun:google.de:3478", SchemeTypeSTUN, false, "google.de", 3478, ProtoTypeUDP},
			{"stun:google.de:1234", "stun:google.de:1234", SchemeTypeSTUN, false, "google.de", 1234, ProtoTypeUDP},
			{"stuns:google.de", "stuns:google.de:5349", SchemeTypeSTUNS, true, "google.de", 5349, ProtoTypeTCP},
			{"stun:[::1]:123", "stun:[::1]:123", SchemeTypeSTUN, false, "::1", 123, ProtoTypeUDP},
			{"turn:google.de", "turn:google.de:3478?transport=udp", SchemeTypeTURN, false, "google.de", 3478, ProtoTypeUDP},
			{"turns:google.de", "turns:google.de:5349?transport=tcp", SchemeTypeTURNS, true, "google.de", 5349, ProtoTypeTCP},
			{"turn:google.de?transport=udp", "turn:google.de:3478?transport=udp", SchemeTypeTURN, false, "google.de", 3478, ProtoTypeUDP},
			{"turns:google.de?transport=tcp", "turns:google.de:5349?transport=tcp", SchemeTypeTURNS, true, "google.de", 5349, ProtoTypeTCP},
		}

		for i, testCase := range testCases {
			url, err := ParseURL(testCase.rawURL)
			assert.Nil(t, err, "testCase: %d %v", i, testCase)
			if err != nil {
				return
			}

			assert.Equal(t, testCase.expectedScheme, url.Scheme, "testCase: %d %v", i, testCase)
			assert.Equal(t, testCase.expectedURLString, url.String(), "testCase: %d %v", i, testCase)
			assert.Equal(t, testCase.expectedSecure, url.IsSecure(), "testCase: %d %v", i, testCase)
			assert.Equal(t, testCase.expectedHost, url.Host, "testCase: %d %v", i, testCase)
			assert.Equal(t, testCase.expectedPort, url.Port, "testCase: %d %v", i, testCase)
			assert.Equal(t, testCase.expectedProto, url.Proto, "testCase: %d %v", i, testCase)
		}
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			rawURL      string
			expectedErr error
		}{
			{"", &SyntaxError{ErrSchemeType}},
			{":::", &UnknownError{errors.New("parse :::: missing protocol scheme")}},
			{"stun:[::1]:123:", &UnknownError{errors.New("address [::1]:123:: too many colons in address")}},
			{"stun:[::1]:123a", &SyntaxError{ErrPort}},
			{"google.de", &SyntaxError{ErrSchemeType}},
			{"stun:", &SyntaxError{ErrHost}},
			{"stun:google.de:abc", &SyntaxError{ErrPort}},
			{"stun:google.de?transport=udp", &SyntaxError{ErrSTUNQuery}},
			{"stuns:google.de?transport=udp", &SyntaxError{ErrSTUNQuery}},
			{"turn:google.de?trans=udp", &SyntaxError{ErrInvalidQuery}},
			{"turns:google.de?trans=udp", &SyntaxError{ErrInvalidQuery}},
			{"turns:google.de?transport=udp&another=1", &SyntaxError{ErrInvalidQuery}},
			{"turn:google.de?transport=ip", &NotSupportedError{ErrProtoType}},
		}

		for i, testCase := range testCases {
			_, err := ParseURL(testCase.rawURL)
			assert.EqualError(t, err, testCase.expectedErr.Error(), "testCase: %d %v", i, testCase)
		}
	})
}
