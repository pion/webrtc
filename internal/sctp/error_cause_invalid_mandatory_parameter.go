package sctp

// ErrorCauseInvalidMandatoryParameter represents an SCTP error cause
type ErrorCauseInvalidMandatoryParameter struct {
	ErrorCauseHeader
}

// Marshal populates a []byte from a struct
func (e *ErrorCauseInvalidMandatoryParameter) Marshal() ([]byte, error) {
	return e.ErrorCauseHeader.Marshal()
}

// Unmarshal populates a ErrorCauseUnrecognizedChunkType from raw data
func (e *ErrorCauseInvalidMandatoryParameter) Unmarshal(raw []byte) error {
	return e.ErrorCauseHeader.Unmarshal(raw)
}
