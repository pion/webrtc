package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSDPType(t *testing.T) {
	testCases := []struct {
		sdpTypeString   string
		expectedSDPType SDPType
	}{
		{unknownStr, SDPType(Unknown)},
		{"offer", SDPTypeOffer},
		{"pranswer", SDPTypePranswer},
		{"answer", SDPTypeAnswer},
		{"rollback", SDPTypeRollback},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedSDPType,
			newSDPType(testCase.sdpTypeString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSDPType_String(t *testing.T) {
	testCases := []struct {
		sdpType        SDPType
		expectedString string
	}{
		{SDPType(Unknown), unknownStr},
		{SDPTypeOffer, "offer"},
		{SDPTypePranswer, "pranswer"},
		{SDPTypeAnswer, "answer"},
		{SDPTypeRollback, "rollback"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.sdpType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
