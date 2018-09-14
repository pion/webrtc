package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCIceComponent(t *testing.T) {
	testCases := []struct {
		componentString   string
		expectedComponent RTCIceComponent
	}{
		{"unknown", RTCIceComponent(Unknown)},
		{"rtp", RTCIceComponentRtp},
		{"rtcp", RTCIceComponentRtcp},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			newRTCIceComponent(testCase.componentString),
			testCase.expectedComponent,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceComponent_String(t *testing.T) {
	testCases := []struct {
		state          RTCIceComponent
		expectedString string
	}{
		{RTCIceComponent(Unknown), "unknown"},
		{RTCIceComponentRtp, "rtp"},
		{RTCIceComponentRtcp, "rtcp"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.state.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
