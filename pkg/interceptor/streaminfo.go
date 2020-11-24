// +build !js

package interceptor

// RTPHeaderExtension represents a negotiated RFC5285 RTP header extension.
type RTPHeaderExtension struct {
	URI string
	ID  int
}

// StreamInfo is the Context passed when a StreamLocal or StreamRemote has been Binded or Unbinded
type StreamInfo struct {
	ID                  string
	Attributes          Attributes
	SSRC                uint32
	PayloadType         uint8
	RTPHeaderExtensions []RTPHeaderExtension
}
