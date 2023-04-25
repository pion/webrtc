// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionDescription_JSON(t *testing.T) {
	testCases := []struct {
		desc           SessionDescription
		expectedString string
		unmarshalErr   error
	}{
		{SessionDescription{Type: SDPTypeOffer, SDP: "sdp"}, `{"type":"offer","sdp":"sdp"}`, nil},
		{SessionDescription{Type: SDPTypePranswer, SDP: "sdp"}, `{"type":"pranswer","sdp":"sdp"}`, nil},
		{SessionDescription{Type: SDPTypeAnswer, SDP: "sdp"}, `{"type":"answer","sdp":"sdp"}`, nil},
		{SessionDescription{Type: SDPTypeRollback, SDP: "sdp"}, `{"type":"rollback","sdp":"sdp"}`, nil},
		{SessionDescription{Type: SDPType(Unknown), SDP: "sdp"}, `{"type":"unknown","sdp":"sdp"}`, ErrUnknownType},
	}

	for i, testCase := range testCases {
		descData, err := json.Marshal(testCase.desc)
		assert.Nil(t,
			err,
			"testCase: %d %v marshal err: %v", i, testCase, err,
		)

		assert.Equal(t,
			string(descData),
			testCase.expectedString,
			"testCase: %d %v", i, testCase,
		)

		var desc SessionDescription
		err = json.Unmarshal(descData, &desc)

		if testCase.unmarshalErr != nil {
			assert.Equal(t,
				err,
				testCase.unmarshalErr,
				"testCase: %d %v", i, testCase,
			)
			continue
		}

		assert.Nil(t,
			err,
			"testCase: %d %v unmarshal err: %v", i, testCase, err,
		)

		assert.Equal(t,
			desc,
			testCase.desc,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSessionDescription_Unmarshal(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)
	desc := SessionDescription{
		Type: offer.Type,
		SDP:  offer.SDP,
	}
	assert.Nil(t, desc.parsed)
	parsed1, err := desc.Unmarshal()
	assert.NotNil(t, parsed1)
	assert.NotNil(t, desc.parsed)
	assert.NoError(t, err)
	parsed2, err2 := desc.Unmarshal()
	assert.NotNil(t, parsed2)
	assert.NoError(t, err2)
	assert.NoError(t, pc.Close())

	// check if the two parsed results _really_ match, could be affected by internal caching
	assert.True(t, reflect.DeepEqual(parsed1, parsed2))
}
