package sctp

type ParamRandom struct {
	ParamHeader
	RandomData []byte
}

func (r *ParamRandom) Marshal() ([]byte, error) {
	r.typ = Random
	r.raw = r.RandomData
	return r.ParamHeader.Marshal()
}

func (r *ParamRandom) Unmarshal(raw []byte) (Param, error) {
	r.ParamHeader.Unmarshal(raw)
	r.RandomData = r.raw
	return r, nil
}
