package quic

import (
	quic "github.com/pions/quic-go"
	"github.com/pions/webrtc/pkg/quic/internal/wrapper"
)

// BidirectionalStream represents a bidirectional Quic stream.
// TODO: Split to QuicWritableStream and QuicReadableStream.
type BidirectionalStream struct {
	s *wrapper.Stream
}

// Write writes data to the stream.
func (s *BidirectionalStream) Write(data StreamWriteParameters) error {
	_, err := s.s.WriteQuic(data.Data, data.Finished)
	return err
}

// ReadInto reads from the QuicReadableStream into the buffer.
func (s *BidirectionalStream) ReadInto(data []byte) (StreamReadResult, error) {
	n, fin, err := s.s.ReadQuic(data)
	return StreamReadResult{
		Amount:   n,
		Finished: fin,
	}, err
}

// StreamID returns the ID of the QuicStream
func (s *BidirectionalStream) StreamID() uint64 {
	return s.s.StreamID()
}

// Detach detaches the underlying quic-go stream
func (s *BidirectionalStream) Detach() quic.Stream {
	return s.s.Detach()
}
