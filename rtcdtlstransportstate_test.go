package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCDtlsTransportState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState RTCDtlsTransportState
	}{
		{"unknown", RTCDtlsTransportState(Unknown)},
		{"new", RTCDtlsTransportStateNew},
		{"connecting", RTCDtlsTransportStateConnecting},
		{"connected", RTCDtlsTransportStateConnected},
		{"closed", RTCDtlsTransportStateClosed},
		{"failed", RTCDtlsTransportStateFailed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTCDtlsTransportState(testCase.stateString),
			testCase.expectedState,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCDtlsTransportState_String(t *testing.T) {
	testCases := []struct {
		state          RTCDtlsTransportState
		expectedString string
	}{
		{RTCDtlsTransportState(Unknown), "unknown"},
		{RTCDtlsTransportStateNew, "new"},
		{RTCDtlsTransportStateConnecting, "connecting"},
		{RTCDtlsTransportStateConnected, "connected"},
		{RTCDtlsTransportStateClosed, "closed"},
		{RTCDtlsTransportStateFailed, "failed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.state.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
