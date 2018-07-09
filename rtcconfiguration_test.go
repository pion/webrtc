package webrtc

import "testing"

func TestRTCICEServer_isStun(t *testing.T) {
	testCases := []struct {
		expectedType RTCServerType
		server       RTCICEServer
	}{
		{RTCServerTypeSTUN, RTCICEServer{URLs: []string{"stun:google.de"}}},
		{RTCServerTypeTURN, RTCICEServer{URLs: []string{"turn:google.de"}}},
		{RTCServerTypeUnknown, RTCICEServer{URLs: []string{"google.de"}}},
	}

	for _, testCase := range testCases {
		if serverType := testCase.server.serverType(); serverType != testCase.expectedType {
			t.Errorf("Expected %q to be %s, but got %s", testCase.server.URLs, testCase.expectedType, serverType)
		}
	}
}

func TestPortAndHost(t *testing.T) {
	testCases := []struct {
		url              string
		expectedHost     string
		expectedProtocol string
	}{
		{"stun:stun.l.google.com:19302", "stun.l.google.com:19302", "udp"},
		{"stuns:stun.l.google.com:19302", "stun.l.google.com:19302", "tcp"},
	}

	for _, testCase := range testCases {
		proto, host, err := protocolAndHost(testCase.url)
		if err != nil {
			t.Fatalf("unable to get proto and host: %v", err)
		}
		if proto != testCase.expectedProtocol {
			t.Fatalf("expected %s, got %s", testCase.expectedProtocol, proto)
		}
		if host != testCase.expectedHost {
			t.Fatalf("expected %s, got %s", testCase.expectedHost, host)
		}
	}
}
