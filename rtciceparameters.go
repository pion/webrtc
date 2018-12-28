package webrtc

// RTCIceParameters includes the ICE username fragment
// and password and other ICE-related parameters.
type RTCIceParameters struct {
	UsernameFragment string `json:"usernameFragment"`
	Password         string `json:"password"`
	IceLite          bool   `json:"iceLite"`
}
