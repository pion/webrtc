package sctp

import "encoding/binary"

type ParamHeader struct {
	typ    ParamType
	length int
	raw    []byte
}

func (p *ParamHeader) Marshal(t ParamType, r []byte) ([]byte, error) {
	paramLengthPlusHeader := 4 + len(r)
	padding := getPadding(paramLengthPlusHeader, 4)

	rawParam := make([]byte, paramLengthPlusHeader+padding)
	binary.BigEndian.PutUint16(rawParam[0:], uint16(t))
	binary.BigEndian.PutUint16(rawParam[2:], uint16(paramLengthPlusHeader))
	copy(rawParam[4:], r)

	return rawParam, nil
}

func (p *ParamHeader) Unmarshal(raw []byte) {
	p.typ = ParamType(binary.BigEndian.Uint16(raw[0:]))
	paramLengthPlusHeader := binary.BigEndian.Uint16(raw[2:])
	paramLengthPlusPadding := paramLengthPlusHeader + getParamPadding(paramLengthPlusHeader, 4)
	paramLength := paramLengthPlusHeader - initOptionalVarHeaderLength

	p.raw = raw[4 : 4+paramLength]
	p.length = int(paramLengthPlusPadding)
}

func (p *ParamHeader) Length() int {
	return p.length
}
