package sctp

import "encoding/binary"

type ParamHeader struct {
	typ    ParamType
	length int
	raw    []byte
}

func (p *ParamHeader) Marshal() ([]byte, error) {
	paramLengthPlusHeader := 4 + len(p.raw)
	padding := getPadding(paramLengthPlusHeader, 4)

	rawParam := make([]byte, paramLengthPlusHeader+padding)
	binary.BigEndian.PutUint16(rawParam[0:], uint16(p.typ))
	binary.BigEndian.PutUint16(rawParam[2:], uint16(paramLengthPlusHeader))
	copy(rawParam[4:], p.raw)

	return rawParam, nil
}

func (p *ParamHeader) Unmarshal(raw []byte) {
	paramLengthPlusHeader := binary.BigEndian.Uint16(raw[2:])
	paramLengthPlusPadding := paramLengthPlusHeader + getParamPadding(paramLengthPlusHeader, 4)
	paramLength := paramLengthPlusHeader - initOptionalVarHeaderLength

	p.typ = ParamType(binary.BigEndian.Uint16(raw[0:]))
	p.raw = raw[4 : 4+paramLength]
	p.length = int(paramLengthPlusPadding)
}

func (p *ParamHeader) Length() int {
	return p.length
}
