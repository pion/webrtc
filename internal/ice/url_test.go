package ice

import (
	"errors"
	"testing"

	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
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
			{"", &rtcerr.SyntaxError{Err: ErrSchemeType}},
			{":::", &rtcerr.UnknownError{Err: errors.New("parse :::: missing protocol scheme")}},
			{"stun:[::1]:123:", &rtcerr.UnknownError{Err: errors.New("address [::1]:123:: too many colons in address")}},
			{"stun:[::1]:123a", &rtcerr.SyntaxError{Err: ErrPort}},
			{"google.de", &rtcerr.SyntaxError{Err: ErrSchemeType}},
			{"stun:", &rtcerr.SyntaxError{Err: ErrHost}},
			{"stun:google.de:abc", &rtcerr.SyntaxError{Err: ErrPort}},
			{"stun:google.de?transport=udp", &rtcerr.SyntaxError{Err: ErrSTUNQuery}},
			{"stuns:google.de?transport=udp", &rtcerr.SyntaxError{Err: ErrSTUNQuery}},
			{"turn:google.de?trans=udp", &rtcerr.SyntaxError{Err: ErrInvalidQuery}},
			{"turns:google.de?trans=udp", &rtcerr.SyntaxError{Err: ErrInvalidQuery}},
			{"turns:google.de?transport=udp&another=1", &rtcerr.SyntaxError{Err: ErrInvalidQuery}},
			{"turn:google.de?transport=ip", &rtcerr.NotSupportedError{Err: ErrProtoType}},
		}

		for i, testCase := range testCases {
			_, err := ParseURL(testCase.rawURL)
			assert.EqualError(t, err, testCase.expectedErr.Error(), "testCase: %d %v", i, testCase)
		}
	})
}
