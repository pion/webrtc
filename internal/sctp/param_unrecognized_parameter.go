package sctp

type ParamUnrecognizedParameter struct {
	ParamHeader
	RawParams []byte
}

func (f *ParamUnrecognizedParameter) Marshal() ([]byte, error) {
	f.typ = UnrecognizedParam
	f.raw = f.RawParams
	return f.ParamHeader.Marshal()
}

func (f *ParamUnrecognizedParameter) Unmarshal(raw []byte) (Param, error) {
	f.ParamHeader.Unmarshal(raw)
	f.RawParams = f.raw
	return f, nil
}
