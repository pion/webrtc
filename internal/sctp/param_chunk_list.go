package sctp

type paramChunkList struct {
	paramHeader
	chunkTypes []chunkType
}

func (c *paramChunkList) marshal() ([]byte, error) {
	c.typ = chunkList
	c.raw = make([]byte, len(c.chunkTypes))
	for i, t := range c.chunkTypes {
		c.raw[i] = byte(t)
	}

	return c.paramHeader.marshal()
}

func (c *paramChunkList) unmarshal(raw []byte) (param, error) {
	c.paramHeader.unmarshal(raw)
	for _, t := range c.raw {
		c.chunkTypes = append(c.chunkTypes, chunkType(t))
	}

	return c, nil
}
