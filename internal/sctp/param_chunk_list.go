package sctp

type ParamChunkList struct {
	ParamHeader
	ChunkTypes []ChunkType
}

func (c *ParamChunkList) Marshal() ([]byte, error) {
	c.typ = ChunkList
	c.raw = make([]byte, len(c.ChunkTypes))
	for i, t := range c.ChunkTypes {
		c.raw[i] = byte(t)
	}

	return c.ParamHeader.Marshal()
}

func (c *ParamChunkList) Unmarshal(raw []byte) (Param, error) {
	c.ParamHeader.Unmarshal(raw)
	for _, t := range c.raw {
		c.ChunkTypes = append(c.ChunkTypes, ChunkType(t))
	}

	return c, nil
}

func (c *ParamChunkList) Types() []ChunkType { return c.Types() }
