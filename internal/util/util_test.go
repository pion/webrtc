// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package util //nolint: revive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMathRandAlpha(t *testing.T) {
	assert.Len(t, MathRandAlpha(10), 10, "MathRandAlpha should return 10 characters")
	assert.Regexp(t, `^[a-zA-Z]+$`, MathRandAlpha(10), "MathRandAlpha should be Alpha only")
}
