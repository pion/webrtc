package util

import (
	"regexp"
	"testing"
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
