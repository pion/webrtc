package logger

import "testing"

var _ Logger = (*Optional)(nil)

func TestNewOptional(t *testing.T) {
	testCases := []struct {
		logger Logger
		result *Optional
	}{
		{logger: nil, result: nil},
	}

	for _, testCase := range testCases {
		res := NewOptional(testCase.logger)
		if res != testCase.result {
			t.Errorf("%v != %v", res, testCase.result)
			return
		}
	}
}
