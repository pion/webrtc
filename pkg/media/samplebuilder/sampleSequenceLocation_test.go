// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package samplebuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSampleSequenceLocationCompare(t *testing.T) {
	s1 := sampleSequenceLocation{32, 42}
	assert.Equal(t, slCompareBefore, s1.compare(16))
	assert.Equal(t, slCompareInside, s1.compare(32))
	assert.Equal(t, slCompareInside, s1.compare(38))
	assert.Equal(t, slCompareInside, s1.compare(41))
	assert.Equal(t, slCompareAfter, s1.compare(42))
	assert.Equal(t, slCompareAfter, s1.compare(0x57))

	s2 := sampleSequenceLocation{0xffa0, 32}
	assert.Equal(t, slCompareBefore, s2.compare(0xff00))
	assert.Equal(t, slCompareInside, s2.compare(0xffa0))
	assert.Equal(t, slCompareInside, s2.compare(0xffff))
	assert.Equal(t, slCompareInside, s2.compare(0))
	assert.Equal(t, slCompareInside, s2.compare(31))
	assert.Equal(t, slCompareAfter, s2.compare(32))
	assert.Equal(t, slCompareAfter, s2.compare(128))
}
