package sctp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// errorCauseHeader represents the shared header that is shared by all error causes
type errorCauseHeader struct {
	code errorCauseCode
	len  uint16
}

func (e *errorCauseHeader) marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}

func (e *errorCauseHeader) unmarshal(raw []byte) error {
	e.code = errorCauseCode(binary.BigEndian.Uint16(raw[0:]))
	e.len = binary.BigEndian.Uint16(raw[2:])
	return nil
}

func (e *errorCauseHeader) length() uint16 {
	return e.len
}

func (e *errorCauseHeader) errorCauseCode() errorCauseCode {
	return e.code
}
