package webrtc

import (
	"testing"

	"github.com/pions/webrtc/pkg/ice"
	"github.com/stretchr/testify/assert"
)

func TestRTCIceTransportState_String(t *testing.T) {
	testCases := []struct {
		state          RTCIceTransportState
		expectedString string
	}{
		{RTCIceTransportState(Unknown), unknownStr},
		{RTCIceTransportStateNew, "new"},
		{RTCIceTransportStateChecking, "checking"},
		{RTCIceTransportStateConnected, "connected"},
		{RTCIceTransportStateCompleted, "completed"},
		{RTCIceTransportStateFailed, "failed"},
		{RTCIceTransportStateDisconnected, "disconnected"},
		{RTCIceTransportStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceTransportState_Convert(t *testing.T) {
	testCases := []struct {
		native RTCIceTransportState
		ice    ice.ConnectionState
	}{
		{RTCIceTransportState(Unknown), ice.ConnectionState(Unknown)},
		{RTCIceTransportStateNew, ice.ConnectionStateNew},
		{RTCIceTransportStateChecking, ice.ConnectionStateChecking},
		{RTCIceTransportStateConnected, ice.ConnectionStateConnected},
		{RTCIceTransportStateCompleted, ice.ConnectionStateCompleted},
		{RTCIceTransportStateFailed, ice.ConnectionStateFailed},
		{RTCIceTransportStateDisconnected, ice.ConnectionStateDisconnected},
		{RTCIceTransportStateClosed, ice.ConnectionStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.native.toICE(),
			testCase.ice,
			"testCase: %d %v", i, testCase,
		)
		assert.Equal(t,
			testCase.native,
			newRTCIceTransportStateFromICE(testCase.ice),
			"testCase: %d %v", i, testCase,
		)
	}
}
