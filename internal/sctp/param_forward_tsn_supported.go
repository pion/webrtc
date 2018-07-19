package sctp

type paramForwardTSNSupported struct {
	paramHeader
	chunkTypes []chunkType
}

func (f *paramForwardTSNSupported) marshal() ([]byte, error) {
	f.typ = forwardTSNSupp
	f.raw = []byte{}
	return f.paramHeader.marshal()
}

func (f *paramForwardTSNSupported) unmarshal(raw []byte) (param, error) {
	f.paramHeader.unmarshal(raw)
	return f, nil
}
