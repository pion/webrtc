package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCIceCandidateType(t *testing.T) {
	testCases := []struct {
		typeString   string
		shouldFail   bool
		expectedType RTCIceCandidateType
	}{
		{"unknown", true, RTCIceCandidateType(Unknown)},
		{"host", false, RTCIceCandidateTypeHost},
		{"srflx", false, RTCIceCandidateTypeSrflx},
		{"prflx", false, RTCIceCandidateTypePrflx},
		{"relay", false, RTCIceCandidateTypeRelay},
	}

	for i, testCase := range testCases {
		actual, err := newRTCIceCandidateType(testCase.typeString)
		if (err != nil) != testCase.shouldFail {
			t.Error(err)
		}
		assert.Equal(t,
			testCase.expectedType,
			actual,
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
