package webrtc

import (
	"encoding/json"
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
