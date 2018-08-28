package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCSdpType(t *testing.T) {
	testCases := []struct {
		sdpTypeString   string
		expectedSdpType RTCSdpType
	}{
		{"unknown", RTCSdpType(Unknown)},
		{"offer", RTCSdpTypeOffer},
		{"pranswer", RTCSdpTypePranswer},
		{"answer", RTCSdpTypeAnswer},
		{"rollback", RTCSdpTypeRollback},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			NewRTCSdpType(testCase.sdpTypeString),
			testCase.expectedSdpType,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCSdpType_String(t *testing.T) {
	testCases := []struct {
		sdpType        RTCSdpType
		expectedString string
	}{
		{RTCSdpType(Unknown), "unknown"},
		{RTCSdpTypeOffer, "offer"},
		{RTCSdpTypePranswer, "pranswer"},
		{RTCSdpTypeAnswer, "answer"},
		{RTCSdpTypeRollback, "rollback"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.sdpType.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
