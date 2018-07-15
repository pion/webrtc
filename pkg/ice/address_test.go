package ice

import "testing"

func TestNewURL(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			rawURL                string
			expectedType          ServerType
			expectedSecure        bool
			expectedHost          string
			expectedPort          int
			expectedTransportType TransportType
		}{
			{"stun:google.de", ServerTypeSTUN, false, "google.de", 3478, TransportUDP},
			{"stun:google.de:1234", ServerTypeSTUN, false, "google.de", 1234, TransportUDP},
			{"stuns:google.de", ServerTypeSTUN, true, "google.de", 5349, TransportTCP},
			{"stun:[::1]:123", ServerTypeSTUN, false, "::1", 123, TransportUDP},
			{"turn:google.de", ServerTypeTURN, false, "google.de", 3478, TransportUDP},
			{"turns:google.de", ServerTypeTURN, true, "google.de", 5349, TransportTCP},
			{"turn:google.de?transport=udp", ServerTypeTURN, false, "google.de", 3478, TransportUDP},
			{"turn:google.de?transport=tcp", ServerTypeTURN, false, "google.de", 3478, TransportTCP},
		}

		for i, testCase := range testCases {
			url, err := NewURL(testCase.rawURL)
			if err != nil {
				t.Errorf("Case %d: got error: %v", i, err)
			}

			if url.Type != testCase.expectedType ||
				url.Secure != testCase.expectedSecure ||
				url.Host != testCase.expectedHost ||
				url.Port != testCase.expectedPort ||
				url.TransportType != testCase.expectedTransportType {
				t.Errorf("Case %d: got %s %t %s %d %s",
					i,
					url.Type,
					url.Secure,
					url.Host,
					url.Port,
					url.TransportType,
				)
			}
		}
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			rawURL      string
			expectedErr error
		}{
			{"", ErrServerType},
			{":::", ErrServerType},
			{"google.de", ErrServerType},
			{"stun:", ErrHost},
			{"stun:google.de:abc", ErrPort},
			{"stun:google.de?transport=udp", ErrSTUNQuery},
			{"turn:google.de?trans=udp", ErrInvalidQuery},
			{"turn:google.de?transport=ip", ErrTransportType},
		}

		for i, testCase := range testCases {
			if _, err := NewURL(testCase.rawURL); err != testCase.expectedErr {
				t.Errorf("Case %d: got error '%v' expected '%v'", i, err, testCase.expectedErr)
			}
		}
	})
}
