package webrtc

// func TestGenerateDataChannelID(t *testing.T) {
// 	testCases := []struct {
// 		client bool
// 		c      *RTCPeerConnection
// 		result uint16
// 	}{
// 		{true, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{}}, 0},
// 		{true, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{1: nil}}, 0},
// 		{true, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{0: nil}}, 2},
// 		{true, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{0: nil, 2: nil}}, 4},
// 		{true, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{0: nil, 4: nil}}, 2},
// 		{false, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{}}, 1},
// 		{false, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{0: nil}}, 1},
// 		{false, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{1: nil}}, 3},
// 		{false, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{1: nil, 3: nil}}, 5},
// 		{false, &RTCPeerConnection{sctpTransport: newRTCSctpTransport(), dataChannels: map[uint16]*RTCDataChannel{1: nil, 5: nil}}, 3},
// 	}
//
// 	for _, testCase := range testCases {
// 		id, err := testCase.c.generateDataChannelID(testCase.client)
// 		if err != nil {
// 			t.Errorf("failed to generate id: %v", err)
// 			return
// 		}
// 		if *id != testCase.result {
// 			t.Errorf("Wrong id: %d expected %d", id, testCase.result)
// 		}
// 	}
// }
