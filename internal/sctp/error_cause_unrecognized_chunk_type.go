package sctp

// ErrorCauseUnrecognizedChunkType represents an SCTP error cause
type ErrorCauseUnrecognizedChunkType struct {
	ErrorCauseHeader
}

// Marshal populates a []byte from a struct
func (e *ErrorCauseUnrecognizedChunkType) Marshal() ([]byte, error) {
	return e.ErrorCauseHeader.Marshal()
}

// Unmarshal populates a ErrorCauseUnrecognizedChunkType from raw data
func (e *ErrorCauseUnrecognizedChunkType) Unmarshal(raw []byte) error {
	return e.ErrorCauseHeader.Unmarshal(raw)
}
