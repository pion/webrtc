package sctp

import "github.com/pkg/errors"

type ParamSupportedExtensions struct {
	Raw        []byte
	ChunkTypes []ChunkType
}

func (s *ParamSupportedExtensions) Marshal() ([]byte, error) {
	return nil, errors.New("Not implemented")
}

func (s *ParamSupportedExtensions) Unmarshal(raw []byte) (Param, error) {
	s.Raw = raw
	for t := range raw {
		s.ChunkTypes = append(s.ChunkTypes, ChunkType(t))
	}

	return s, nil
}

func (s *ParamSupportedExtensions) Types() []ChunkType { return s.Types() }
