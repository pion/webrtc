package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTransportPolicy(t *testing.T) {
	testCases := []struct {
		policyString   string
		expectedPolicy TransportPolicy
	}{
		{"relay", TransportPolicyRelay},
		{"all", TransportPolicyAll},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedPolicy,
			NewTransportPolicy(testCase.policyString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestTransportPolicy_String(t *testing.T) {
	testCases := []struct {
		policy         TransportPolicy
		expectedString string
	}{
		{TransportPolicyRelay, "relay"},
		{TransportPolicyAll, "all"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.policy.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}
