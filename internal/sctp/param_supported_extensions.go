package sctp

import (
	"fmt"
)

func chunkTypeIntersect(l, r []ChunkType) (c []ChunkType) {
	m := make(map[ChunkType]bool)

	for _, ct := range l {
		m[ct] = true
	}

	for _, ct := range r {
		if _, ok := m[ct]; ok {
			c = append(c, ct)
		}
	}
	return
}

type ParamSupportedExtensions struct {
	ParamHeader
	Raw        []byte
	ChunkTypes []ChunkType
}

func (s *ParamSupportedExtensions) Marshal() ([]byte, error) {
	r := make([]byte, len(s.ChunkTypes))
	for i, c := range s.ChunkTypes {
		r[i] = byte(c)
	}

	return s.ParamHeader.Marshal(SupportedExt, r)
}

func (s *ParamSupportedExtensions) Unmarshal(raw []byte) (Param, error) {
	s.ParamHeader.Unmarshal(raw)

	for t := range s.raw {
		s.ChunkTypes = append(s.ChunkTypes, ChunkType(t))
	}
	fmt.Print(s.ChunkTypes)

	return s, nil
}

func (s *ParamSupportedExtensions) Types() []ChunkType { return s.Types() }
