package sctp

import "github.com/pkg/errors"

type ParamRandom struct {
	Raw []byte
}

func (r *ParamRandom) Marshal() ([]byte, error) {
	return nil, errors.New("Not implemented")
}

func (r *ParamRandom) Unmarshal(raw []byte) (Param, error) {
	r.Raw = raw

	return r, nil
}

func (r *ParamRandom) Value() []byte { return r.Raw }
