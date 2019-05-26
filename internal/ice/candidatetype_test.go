package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCandidateType(t *testing.T) {
	testCases := []struct {
		typeString   string
		shouldFail   bool
		expectedType CandidateType
	}{
		{unknownStr, true, CandidateType(Unknown)},
		{"host", false, CandidateTypeHost},
		{"srflx", false, CandidateTypeSrflx},
		{"prflx", false, CandidateTypePrflx},
		{"relay", false, CandidateTypeRelay},
	}

	for i, testCase := range testCases {
		actual, err := NewCandidateType(testCase.typeString)
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

func TestCandidateType_String(t *testing.T) {
	testCases := []struct {
		cType          CandidateType
		expectedString string
	}{
		{CandidateType(Unknown), unknownStr},
		{CandidateTypeHost, "host"},
		{CandidateTypeSrflx, "srflx"},
		{CandidateTypePrflx, "prflx"},
		{CandidateTypeRelay, "relay"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.cType.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
