package util

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandSeq(t *testing.T) {
	if len(RandSeq(10)) != 10 {
		t.Errorf("RandSeq return invalid length")
	}

	var isLetter = regexp.MustCompile(`^[a-zA-Z]+$`).MatchString
	if !isLetter(RandSeq(10)) {
		t.Errorf("RandSeq should be AlphaNumeric only")
	}
}

func TestGetPadding(t *testing.T) {
	assert := assert.New(t)
	type testCase struct {
		input  int
		result int
	}

	cases := []testCase{
		{input: 0, result: 0},
		{input: 1, result: 3},
		{input: 2, result: 2},
		{input: 3, result: 1},
		{input: 4, result: 0},
		{input: 100, result: 0},
		{input: 500, result: 0},
	}
	for _, testCase := range cases {
		assert.Equalf(GetPadding(testCase.input), testCase.result, "Test case returned wrong value for input %d", testCase.input)
	}
}
