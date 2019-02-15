package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewICERole(t *testing.T) {
	testCases := []struct {
		roleString   string
		expectedRole ICERole
	}{
		{unknownStr, ICERole(Unknown)},
		{"controlling", ICERoleControlling},
		{"controlled", ICERoleControlled},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedRole,
			newICERole(testCase.roleString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICERole_String(t *testing.T) {
	testCases := []struct {
		proto          ICERole
		expectedString string
	}{
		{ICERole(Unknown), unknownStr},
		{ICERoleControlling, "controlling"},
		{ICERoleControlled, "controlled"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.proto.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
