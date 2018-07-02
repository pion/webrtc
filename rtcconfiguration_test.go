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
