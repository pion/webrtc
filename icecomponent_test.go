// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestICEComponent(t *testing.T) {
	testCases := []struct {
		componentString   string
		expectedComponent ICEComponent
	}{
		{ErrUnknownType.Error(), ICEComponentUnknown},
		{"rtp", ICEComponentRTP},
		{"rtcp", ICEComponentRTCP},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			newICEComponent(testCase.componentString),
			testCase.expectedComponent,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestICEComponent_String(t *testing.T) {
	testCases := []struct {
		state          ICEComponent
		expectedString string
	}{
		{ICEComponentUnknown, ErrUnknownType.Error()},
		{ICEComponentRTP, "rtp"},
		{ICEComponentRTCP, "rtcp"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.state.String(),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)
	}
}
