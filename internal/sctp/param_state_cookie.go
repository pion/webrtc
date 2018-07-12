package sctp

import (
	"encoding/binary"
	"math/rand"
	"time"
)

type ParamStateCookie struct {
	ParamHeader
	Cookie []byte
}

func NewRandomStateCookie() *ParamStateCookie {
	rs := rand.NewSource(time.Now().UnixNano())
	r := rand.New(rs)
	randCookie := make([]byte, 4)
	binary.BigEndian.PutUint32(randCookie, r.Uint32())
	s := &ParamStateCookie{
		Cookie: randCookie,
	}

	return s
}

func (s *ParamStateCookie) Marshal() ([]byte, error) {
	s.typ = StateCookie
	s.raw = s.Cookie
	return s.ParamHeader.Marshal()
}

func (s *ParamStateCookie) Unmarshal(raw []byte) (Param, error) {
	s.ParamHeader.Unmarshal(raw)
	s.Cookie = s.raw
	return s, nil
}
