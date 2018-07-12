package sctp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// ErrorCauseHeader represents the shared header that is shared by all error causes
type ErrorCauseHeader struct {
	code   ErrorCauseCode
	length uint16
}

// Marshal generates populates a []byte from a ErrorCauseHeader
func (e *ErrorCauseHeader) Marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}

// Unmarshal generates populates ErrorCauseHeader from a []byte
func (e *ErrorCauseHeader) Unmarshal(raw []byte) error {
	e.code = ErrorCauseCode(binary.BigEndian.Uint16(raw[0:]))
	e.length = binary.BigEndian.Uint16(raw[2:])
	return nil
}

// Length returns the total length of the packet after it has been Unmarshaled
func (e *ErrorCauseHeader) Length() uint16 {
	return e.length
}

func (e *ErrorCauseHeader) errorCauseCode() ErrorCauseCode {
	return e.code
}
