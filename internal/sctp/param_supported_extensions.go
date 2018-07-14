package sctp

func chunkTypeIntersect(l, r []ChunkType) (c []ChunkType) {
	m := make(map[ChunkType]bool)

	for _, ct := range l {
		m[ct] = true
	}

	for _, ct := range r {
		if _, ok := m[ct]; ok {
			c = append(c, ct)
		}
	}
	return
}

func NewEmptySupportedExtensions() *ParamSupportedExtensions {
	return &ParamSupportedExtensions{}
}

type ParamSupportedExtensions struct {
	ParamHeader
	ChunkTypes []ChunkType
}

func (s *ParamSupportedExtensions) Marshal() ([]byte, error) {
	s.typ = SupportedExt
	s.raw = make([]byte, len(s.ChunkTypes))
	for i, c := range s.ChunkTypes {
		s.raw[i] = byte(c)
	}

	return s.ParamHeader.Marshal()
}

func (s *ParamSupportedExtensions) Unmarshal(raw []byte) (Param, error) {
	s.ParamHeader.Unmarshal(raw)

	for _, t := range s.raw {
		s.ChunkTypes = append(s.ChunkTypes, ChunkType(t))
	}

	return s, nil
}

func (s *ParamSupportedExtensions) Types() []ChunkType { return s.Types() }
