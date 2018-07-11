package sctp

type ParamForwardTSNSupported struct {
	ParamHeader
	ChunkTypes []ChunkType
}

func (f *ParamForwardTSNSupported) Marshal() ([]byte, error) {
	return f.ParamHeader.Marshal(ForwardTSNSupp, []byte{})
}

func (f *ParamForwardTSNSupported) Unmarshal(raw []byte) (Param, error) {
	f.ParamHeader.Unmarshal(raw)
	return f, nil
}
