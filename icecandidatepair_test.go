// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestICECandidatePairString_Nil(t *testing.T) {
	var pair *ICECandidatePair
	assert.Equal(t, "<nil>", pair.String())
}
