package sctp

type paramUnrecognizedParameter struct {
	paramHeader
	RawParams []byte
}

func (f *paramUnrecognizedParameter) marshal() ([]byte, error) {
	f.typ = unrecognizedParam
	f.raw = f.RawParams
	return f.paramHeader.marshal()
}

func (f *paramUnrecognizedParameter) unmarshal(raw []byte) (param, error) {
	f.paramHeader.unmarshal(raw)
	f.RawParams = f.raw
	return f, nil
}
