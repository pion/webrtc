package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCIceCandidateType(t *testing.T) {
	testCases := []struct {
		typeString   string
		expectedType RTCIceCandidateType
	}{
		{"unknown", RTCIceCandidateType(Unknown)},
		{"host", RTCIceCandidateTypeHost},
		{"srflx", RTCIceCandidateTypeSrflx},
		{"prflx", RTCIceCandidateTypePrflx},
		{"relay", RTCIceCandidateTypeRelay},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedType,
			newRTCIceCandidateType(testCase.typeString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCIceCandidateType_String(t *testing.T) {
	testCases := []struct {
		cType          RTCIceCandidateType
		expectedString string
	}{
		{RTCIceCandidateType(Unknown), "unknown"},
		{RTCIceCandidateTypeHost, "host"},
		{RTCIceCandidateTypeSrflx, "srflx"},
		{RTCIceCandidateTypePrflx, "prflx"},
		{RTCIceCandidateTypeRelay, "relay"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.cType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
