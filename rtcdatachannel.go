package webrtc

// RTCDataChannel represents a WebRTC DataChannel
// The RTCDataChannel interface represents a network channel
// which can be used for bidirectional peer-to-peer transfers of arbitrary data
type RTCDataChannel struct {
	Onmessage func([]byte)
	ID        uint16
	Label     string

	rtcPeerConnection *RTCPeerConnection
}

// Send sends the passed message to the DataChannel peer
func (r *RTCDataChannel) Send(message []byte) error {
	if err := r.rtcPeerConnection.networkManager.SendDataChannelMessage(message, r.ID); err != nil {
		return &UnknownError{Err: err}
	}

	return nil
}
