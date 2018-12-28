package quic

// StreamReadResult holds information relating to the result returned from readInto.
type StreamReadResult struct {
	Amount   int
	Finished bool
}
