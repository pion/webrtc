package webrtc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRTCSessionDescription_JSON(t *testing.T) {
	testCases := []struct {
		desc           RTCSessionDescription
		expectedString string
		unmarshalErr   error
	}{
		{RTCSessionDescription{Type: RTCSdpTypeOffer, Sdp: "sdp"}, `{"type":"offer","sdp":"sdp"}`, nil},
		{RTCSessionDescription{Type: RTCSdpTypePranswer, Sdp: "sdp"}, `{"type":"pranswer","sdp":"sdp"}`, nil},
		{RTCSessionDescription{Type: RTCSdpTypeAnswer, Sdp: "sdp"}, `{"type":"answer","sdp":"sdp"}`, nil},
		{RTCSessionDescription{Type: RTCSdpTypeRollback, Sdp: "sdp"}, `{"type":"rollback","sdp":"sdp"}`, nil},
		{RTCSessionDescription{Type: RTCSdpType(Unknown), Sdp: "sdp"}, `{"type":"unknown","sdp":"sdp"}`, ErrUnknownType},
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

		var desc RTCSessionDescription
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
