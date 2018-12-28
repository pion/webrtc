package webrtc

// RTCDataChannelParameters describes the configuration of the RTCDataChannel.
type RTCDataChannelParameters struct {
	Label string `json:"label"`
	ID    uint16 `json:"id"`
}
