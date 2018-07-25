package sctp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makePayload(tsn uint32) *chunkPayloadData {
	return &chunkPayloadData{tsn: tsn}
}

func TestPayloadQueue_GetGapAckBlocks(t *testing.T) {
	pq := &payloadQueue{}
	pq.push(makePayload(1), 0)
	pq.push(makePayload(2), 0)
	pq.push(makePayload(3), 0)
	pq.push(makePayload(4), 0)
	pq.push(makePayload(5), 0)
	pq.push(makePayload(6), 0)

	gab1 := []*gapAckBlock{{start: 1, end: 6}}
	gab2 := pq.getGapAckBlocks(0)
	assert.NotNil(t, gab2)
	assert.Len(t, gab2, 1)

	assert.Equal(t, gab1[0].start, gab2[0].start)
	assert.Equal(t, gab1[0].end, gab2[0].end)

	pq.push(makePayload(8), 0)
	pq.push(makePayload(9), 0)

	gab1 = []*gapAckBlock{{start: 1, end: 6}, {start: 8, end: 9}}
	gab2 = pq.getGapAckBlocks(0)
	assert.NotNil(t, gab2)
	assert.Len(t, gab2, 2)

	assert.Equal(t, gab1[0].start, gab2[0].start)
	assert.Equal(t, gab1[0].end, gab2[0].end)
	assert.Equal(t, gab1[1].start, gab2[1].start)
	assert.Equal(t, gab1[1].end, gab2[1].end)
}
