package sctp

type ParamRandom struct {
	ParamHeader
	RandomData []byte
}

func (r *ParamRandom) Marshal() ([]byte, error) {
	return r.ParamHeader.Marshal(Random, r.RandomData)
}

func (r *ParamRandom) Unmarshal(raw []byte) (Param, error) {
	r.ParamHeader.Unmarshal(raw)
	r.RandomData = r.raw
	return r, nil
}
