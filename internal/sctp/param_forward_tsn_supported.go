package sctp

import "github.com/pkg/errors"

type ParamForwardTSNSupported struct {
	Raw        []byte
	ChunkTypes []ChunkType
}

func (f *ParamForwardTSNSupported) Marshal() ([]byte, error) {
	return nil, errors.New("Not implemented")
}

func (f *ParamForwardTSNSupported) Unmarshal(raw []byte) (Param, error) {
	f.Raw = raw
	return f, nil
}
