package quic

// StreamWriteParameters holds information relating to the data to be written.
type StreamWriteParameters struct {
	Data     []byte
	Finished bool
}
