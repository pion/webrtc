package rtcp

// Packet represents an RTCP packet, a protocol used for out-of-band statistics and control information for an RTP session
type Packet interface {
	Marshal() ([]byte, error)
	Unmarshal(rawPacket []byte) error
}
