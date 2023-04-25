// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"testing"

	"github.com/pion/webrtc/v3/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

func TestNewSignalingState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState SignalingState
	}{
		{unknownStr, SignalingState(Unknown)},
		{"stable", SignalingStateStable},
		{"have-local-offer", SignalingStateHaveLocalOffer},
		{"have-remote-offer", SignalingStateHaveRemoteOffer},
		{"have-local-pranswer", SignalingStateHaveLocalPranswer},
		{"have-remote-pranswer", SignalingStateHaveRemotePranswer},
		{"closed", SignalingStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newSignalingState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSignalingState_String(t *testing.T) {
	testCases := []struct {
		state          SignalingState
		expectedString string
	}{
		{SignalingState(Unknown), unknownStr},
		{SignalingStateStable, "stable"},
		{SignalingStateHaveLocalOffer, "have-local-offer"},
		{SignalingStateHaveRemoteOffer, "have-remote-offer"},
		{SignalingStateHaveLocalPranswer, "have-local-pranswer"},
		{SignalingStateHaveRemotePranswer, "have-remote-pranswer"},
		{SignalingStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSignalingState_Transitions(t *testing.T) {
	testCases := []struct {
		desc        string
		current     SignalingState
		next        SignalingState
		op          stateChangeOp
		sdpType     SDPType
		expectedErr error
	}{
		{
			"stable->SetLocal(offer)->have-local-offer",
			SignalingStateStable,
			SignalingStateHaveLocalOffer,
			stateChangeOpSetLocal,
			SDPTypeOffer,
			nil,
		},
		{
			"stable->SetRemote(offer)->have-remote-offer",
			SignalingStateStable,
			SignalingStateHaveRemoteOffer,
			stateChangeOpSetRemote,
			SDPTypeOffer,
			nil,
		},
		{
			"have-local-offer->SetRemote(answer)->stable",
			SignalingStateHaveLocalOffer,
			SignalingStateStable,
			stateChangeOpSetRemote,
			SDPTypeAnswer,
			nil,
		},
		{
			"have-local-offer->SetRemote(pranswer)->have-remote-pranswer",
			SignalingStateHaveLocalOffer,
			SignalingStateHaveRemotePranswer,
			stateChangeOpSetRemote,
			SDPTypePranswer,
			nil,
		},
		{
			"have-remote-pranswer->SetRemote(answer)->stable",
			SignalingStateHaveRemotePranswer,
			SignalingStateStable,
			stateChangeOpSetRemote,
			SDPTypeAnswer,
			nil,
		},
		{
			"have-remote-offer->SetLocal(answer)->stable",
			SignalingStateHaveRemoteOffer,
			SignalingStateStable,
			stateChangeOpSetLocal,
			SDPTypeAnswer,
			nil,
		},
		{
			"have-remote-offer->SetLocal(pranswer)->have-local-pranswer",
			SignalingStateHaveRemoteOffer,
			SignalingStateHaveLocalPranswer,
			stateChangeOpSetLocal,
			SDPTypePranswer,
			nil,
		},
		{
			"have-local-pranswer->SetLocal(answer)->stable",
			SignalingStateHaveLocalPranswer,
			SignalingStateStable,
			stateChangeOpSetLocal,
			SDPTypeAnswer,
			nil,
		},
		{
			"(invalid) stable->SetRemote(pranswer)->have-remote-pranswer",
			SignalingStateStable,
			SignalingStateHaveRemotePranswer,
			stateChangeOpSetRemote,
			SDPTypePranswer,
			&rtcerr.InvalidModificationError{},
		},
		{
			"(invalid) stable->SetRemote(rollback)->have-local-offer",
			SignalingStateStable,
			SignalingStateHaveLocalOffer,
			stateChangeOpSetRemote,
			SDPTypeRollback,
			&rtcerr.InvalidModificationError{},
		},
	}

	for i, tc := range testCases {
		next, err := checkNextSignalingState(tc.current, tc.next, tc.op, tc.sdpType)
		if tc.expectedErr != nil {
			assert.Error(t, err, "testCase: %d %s", i, tc.desc)
		} else {
			assert.NoError(t, err, "testCase: %d %s", i, tc.desc)
			assert.Equal(t,
				tc.next,
				next,
				"testCase: %d %s", i, tc.desc,
			)
		}
	}
}
