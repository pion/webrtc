package sctp

import "github.com/pkg/errors"

type ParamChunkList struct {
	Raw        []byte
	ChunkTypes []ChunkType
}

func (c *ParamChunkList) Marshal() ([]byte, error) {
	return nil, errors.New("Not implemented")
}

func (c *ParamChunkList) Unmarshal(raw []byte) (Param, error) {
	c.Raw = raw
	for t := range raw {
		c.ChunkTypes = append(c.ChunkTypes, ChunkType(t))
	}

	return c, nil
}

func (c *ParamChunkList) Types() []ChunkType { return c.Types() }
