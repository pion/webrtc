// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMathRandAlpha(t *testing.T) {
	assert.Len(t, MathRandAlpha(10), 10, "MathRandAlpha should return 10 characters")
	assert.Regexp(t, `^[a-zA-Z]+$`, MathRandAlpha(10), "MathRandAlpha should be Alpha only")
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

	assert.Equal(t, str, errs.Error(), "String representation doesn't match")

	errIs, ok := errs.(multiError) //nolint:errorlint
	assert.True(t, ok, "FlattenErrs returns non-multiError")
	for i := 0; i < 3; i++ {
		assert.Truef(t, errIs.Is(rawErrs[i]), "Should contains this error '%v'", rawErrs[i])
	}

	assert.Falsef(t, errIs.Is(rawErrs[3]), "Should not contains this error '%v'", rawErrs[3])
}
