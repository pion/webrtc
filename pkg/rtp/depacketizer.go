package rtp

// Depacketizer depacketizes a RTP payload, removing any RTP specific data from the payload
type Depacketizer interface {
	Unmarshal(packet *Packet) ([]byte, error)
}
