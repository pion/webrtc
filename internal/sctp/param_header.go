package sctp

import (
	"encoding/binary"
	"fmt"
)

type paramHeader struct {
	typ paramType
	len int
	raw []byte
}

const (
	paramHeaderLength = 4
)

func (p *paramHeader) marshal() ([]byte, error) {
	paramLengthPlusHeader := paramHeaderLength + len(p.raw)

	rawParam := make([]byte, paramLengthPlusHeader)
	binary.BigEndian.PutUint16(rawParam[0:], uint16(p.typ))
	binary.BigEndian.PutUint16(rawParam[2:], uint16(paramLengthPlusHeader))
	copy(rawParam[paramHeaderLength:], p.raw)

	return rawParam, nil
}

func (p *paramHeader) unmarshal(raw []byte) {
	paramLengthPlusHeader := binary.BigEndian.Uint16(raw[2:])
	paramLength := paramLengthPlusHeader - initOptionalVarHeaderLength

	p.typ = paramType(binary.BigEndian.Uint16(raw[0:]))
	p.raw = raw[paramHeaderLength : paramHeaderLength+paramLength]
	p.len = int(paramLengthPlusHeader)
}

func (p *paramHeader) length() int {
	return p.len
}

// String makes paramHeader printable
func (p paramHeader) String() string {
	return fmt.Sprintf("%s (%d): %s", p.typ, p.len, p.raw)
}
