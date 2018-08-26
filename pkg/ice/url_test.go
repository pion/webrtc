package ice

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseURL(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			rawURL         string
			expectedScheme SchemeType
			expectedHost   string
			expectedPort   int
			expectedProto  ProtoType
		}{
			{"stun:google.de", SchemeTypeSTUN, "google.de", 3478, ProtoTypeUDP},
			{"stun:google.de:1234", SchemeTypeSTUN, "google.de", 1234, ProtoTypeUDP},
			{"stuns:google.de", SchemeTypeSTUNS, "google.de", 5349, ProtoTypeTCP},
			{"stun:[::1]:123", SchemeTypeSTUN, "::1", 123, ProtoTypeUDP},
			{"turn:google.de", SchemeTypeTURN, "google.de", 3478, ProtoTypeUDP},
			{"turns:google.de", SchemeTypeTURNS, "google.de", 5349, ProtoTypeTCP},
			{"turn:google.de?transport=udp", SchemeTypeTURN, "google.de", 3478, ProtoTypeUDP},
			{"turn:google.de?transport=tcp", SchemeTypeTURN, "google.de", 3478, ProtoTypeTCP},
		}

		for i, testCase := range testCases {
			url, err := ParseURL(testCase.rawURL)
			assert.Nil(t, err, "testCase: %d %v", i, testCase)
			if err != nil {
				return
			}

			assert.Equal(t, testCase.expectedScheme, url.Scheme, "testCase: %d %v", i, testCase)
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
			{"", SyntaxError{Err: ErrSchemeType}},
			{":::", UnknownError{Err: errors.New("parse :::: missing protocol scheme")}},
			{"google.de", SyntaxError{Err: ErrSchemeType}},
			{"stun:", SyntaxError{Err: ErrHost}},
			{"stun:google.de:abc", SyntaxError{Err: ErrPort}},
			{"stun:google.de?transport=udp", SyntaxError{Err: ErrSTUNQuery}},
			{"turn:google.de?trans=udp", SyntaxError{Err: ErrInvalidQuery}},
			{"turn:google.de?transport=ip", SyntaxError{Err: ErrProtoType}},
		}

		for i, testCase := range testCases {
			_, err := ParseURL(testCase.rawURL)
			assert.EqualError(t, err, testCase.expectedErr.Error(), "testCase: %d %v", i, testCase)
		}
	})
}
