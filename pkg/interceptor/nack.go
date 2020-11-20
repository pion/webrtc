// +build !js

package interceptor

// NACK interceptor generates/responds to nack messages.
type NACK struct {
	NoOp
}

// BindRemoteStream lets you modify any incoming RTP packets. It is called once for per RemoteStream. The returned method
// will be called once per rtp packet.
func (n *NACK) BindRemoteStream(_ *StreamInfo, reader RTPReader) RTPReader {
	return reader
}
