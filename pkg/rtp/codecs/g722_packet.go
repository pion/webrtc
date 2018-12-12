package codecs

// G722Payloader payloads G722 packets
type G722Payloader struct{}

// Payload fragments an G722 packet across one or more byte arrays
func (p *G722Payloader) Payload(mtu int, payload []byte) [][]byte {
	var out [][]byte
	for len(payload) > mtu {
		o := make([]byte, mtu)
		copy(o, payload[:mtu])
		payload = payload[mtu:]
		out = append(out, o)
	}
	o := make([]byte, len(payload))
	copy(o, payload)
	return append(out, o)
}
