package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCSctpTransportState(t *testing.T) {
	testCases := []struct {
		transportStateString   string
		expectedTransportState RTCSctpTransportState
	}{
		{"unknown", RTCSctpTransportState(Unknown)},
		{"connecting", RTCSctpTransportStateConnecting},
		{"connected", RTCSctpTransportStateConnected},
		{"closed", RTCSctpTransportStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedTransportState,
			newRTCSctpTransportState(testCase.transportStateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCSctpTransportState_String(t *testing.T) {
	testCases := []struct {
		transportState RTCSctpTransportState
		expectedString string
	}{
		{RTCSctpTransportState(Unknown), "unknown"},
		{RTCSctpTransportStateConnecting, "connecting"},
		{RTCSctpTransportStateConnected, "connected"},
		{RTCSctpTransportStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.transportState.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
