package sctp

type ParamForwardTSNSupported struct {
	ParamHeader
	ChunkTypes []ChunkType
}

func (f *ParamForwardTSNSupported) Marshal() ([]byte, error) {
	f.typ = ForwardTSNSupp
	f.raw = []byte{}
	return f.ParamHeader.Marshal()
}

func (f *ParamForwardTSNSupported) Unmarshal(raw []byte) (Param, error) {
	f.ParamHeader.Unmarshal(raw)
	return f, nil
}
