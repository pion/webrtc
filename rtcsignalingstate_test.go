package webrtc

import (
	"testing"

	"github.com/pions/webrtc/pkg/rtcerr"

	"github.com/stretchr/testify/assert"
)

func TestNewRTCSignalingState(t *testing.T) {
	testCases := []struct {
		stateString   string
		expectedState RTCSignalingState
	}{
		{"unknown", RTCSignalingState(Unknown)},
		{"stable", RTCSignalingStateStable},
		{"have-local-offer", RTCSignalingStateHaveLocalOffer},
		{"have-remote-offer", RTCSignalingStateHaveRemoteOffer},
		{"have-local-pranswer", RTCSignalingStateHaveLocalPranswer},
		{"have-remote-pranswer", RTCSignalingStateHaveRemotePranswer},
		{"closed", RTCSignalingStateClosed},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedState,
			newRTCSignalingState(testCase.stateString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCSignalingState_String(t *testing.T) {
	testCases := []struct {
		state          RTCSignalingState
		expectedString string
	}{
		{RTCSignalingState(Unknown), "unknown"},
		{RTCSignalingStateStable, "stable"},
		{RTCSignalingStateHaveLocalOffer, "have-local-offer"},
		{RTCSignalingStateHaveRemoteOffer, "have-remote-offer"},
		{RTCSignalingStateHaveLocalPranswer, "have-local-pranswer"},
		{RTCSignalingStateHaveRemotePranswer, "have-remote-pranswer"},
		{RTCSignalingStateClosed, "closed"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.state.String(),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestRTCSignalingState_Transitions(t *testing.T) {
	testCases := []struct {
		desc        string
		current     RTCSignalingState
		next        RTCSignalingState
		op          rtcStateChangeOp
		sdpType     RTCSdpType
		expectedErr error
	}{
		{
			"stable->SetLocal(offer)->have-local-offer",
			RTCSignalingStateStable,
			RTCSignalingStateHaveLocalOffer,
			rtcStateChangeOpSetLocal,
			RTCSdpTypeOffer,
			nil,
		},
		{
			"stable->SetRemote(offer)->have-remote-offer",
			RTCSignalingStateStable,
			RTCSignalingStateHaveRemoteOffer,
			rtcStateChangeOpSetRemote,
			RTCSdpTypeOffer,
			nil,
		},
		{
			"have-local-offer->SetRemote(answer)->stable",
			RTCSignalingStateHaveLocalOffer,
			RTCSignalingStateStable,
			rtcStateChangeOpSetRemote,
			RTCSdpTypeAnswer,
			nil,
		},
		{
			"have-local-offer->SetRemote(pranswer)->have-remote-pranswer",
			RTCSignalingStateHaveLocalOffer,
			RTCSignalingStateHaveRemotePranswer,
			rtcStateChangeOpSetRemote,
			RTCSdpTypePranswer,
			nil,
		},
		{
			"have-remote-pranswer->SetRemote(answer)->stable",
			RTCSignalingStateHaveRemotePranswer,
			RTCSignalingStateStable,
			rtcStateChangeOpSetRemote,
			RTCSdpTypeAnswer,
			nil,
		},
		{
			"have-remote-offer->SetLocal(answer)->stable",
			RTCSignalingStateHaveRemoteOffer,
			RTCSignalingStateStable,
			rtcStateChangeOpSetLocal,
			RTCSdpTypeAnswer,
			nil,
		},
		{
			"have-remote-offer->SetLocal(pranswer)->have-local-pranswer",
			RTCSignalingStateHaveRemoteOffer,
			RTCSignalingStateHaveLocalPranswer,
			rtcStateChangeOpSetLocal,
			RTCSdpTypePranswer,
			nil,
		},
		{
			"have-local-pranswer->SetLocal(answer)->stable",
			RTCSignalingStateHaveLocalPranswer,
			RTCSignalingStateStable,
			rtcStateChangeOpSetLocal,
			RTCSdpTypeAnswer,
			nil,
		},
		{
			"(invalid) stable->SetRemote(pranswer)->have-remote-pranswer",
			RTCSignalingStateStable,
			RTCSignalingStateHaveRemotePranswer,
			rtcStateChangeOpSetRemote,
			RTCSdpTypePranswer,
			&rtcerr.InvalidModificationError{},
		},
		{
			"(invalid) stable->SetRemote(rollback)->have-local-offer",
			RTCSignalingStateStable,
			RTCSignalingStateHaveLocalOffer,
			rtcStateChangeOpSetRemote,
			RTCSdpTypeRollback,
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
