// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"regexp"
	"testing"
)

func TestMathRandAlpha(t *testing.T) {
	if len(MathRandAlpha(10)) != 10 {
		t.Errorf("MathRandAlpha return invalid length")
	}

	isLetter := regexp.MustCompile(`^[a-zA-Z]+$`).MatchString
	if !isLetter(MathRandAlpha(10)) {
		t.Errorf("MathRandAlpha should be AlphaNumeric only")
	}
}

func TestMultiError(t *testing.T) {
	rawErrs := []error{
		errors.New("err1"), //nolint
		errors.New("err2"), //nolint
		errors.New("err3"), //nolint
		errors.New("err4"), //nolint
	}
	errs := FlattenErrs([]error{
		rawErrs[0],
		nil,
		rawErrs[1],
		FlattenErrs([]error{
			rawErrs[2],
		}),
	})
	str := "err1\nerr2\nerr3"

	if errs.Error() != str {
		t.Errorf("String representation doesn't match, expected: %s, got: %s", errs.Error(), str)
	}

	errIs, ok := errs.(multiError) //nolint:errorlint
	if !ok {
		t.Fatal("FlattenErrs returns non-multiError")
	}
	for i := 0; i < 3; i++ {
		if !errIs.Is(rawErrs[i]) {
			t.Errorf("'%+v' should contains '%v'", errs, rawErrs[i])
		}
	}
	if errIs.Is(rawErrs[3]) {
		t.Errorf("'%+v' should not contains '%v'", errs, rawErrs[3])
	}
}
