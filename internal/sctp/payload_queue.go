package sctp

import (
	"sort"
)

type payloadDataArray []*chunkPayloadData

func (s payloadDataArray) search(tsn uint32) (*chunkPayloadData, bool) {
	i := sort.Search(len(s), func(i int) bool {
		return s[i].tsn >= tsn
	})

	if i < len(s) && s[i].tsn == tsn {
		return s[i], true
	}

	return nil, false
}

func (s payloadDataArray) sort() {
	sort.Slice(s, func(i, j int) bool { return s[i].tsn < s[j].tsn })
}

type payloadQueue struct {
	orderedPackets payloadDataArray
	dupTSN         []uint32
}

func (r *payloadQueue) pushNoCheck(p *chunkPayloadData) {
	r.orderedPackets = append(r.orderedPackets, p)
	r.orderedPackets.sort()
}

func (r *payloadQueue) push(p *chunkPayloadData, cumulativeTSN uint32) {
	_, ok := r.orderedPackets.search(p.tsn)

	// If the Data payload is already in our queue or older than our cumulativeTSN marker
	if ok || p.tsn <= cumulativeTSN {
		// Found the packet, log in dups
		r.dupTSN = append(r.dupTSN, p.tsn)
		return
	}

	r.orderedPackets = append(r.orderedPackets, p)
	r.orderedPackets.sort()
}

func (r *payloadQueue) pop(tsn uint32) (*chunkPayloadData, bool) {
	if len(r.orderedPackets) > 0 && tsn == r.orderedPackets[0].tsn {
		pd := r.orderedPackets[0]
		r.orderedPackets = r.orderedPackets[1:]
		return pd, true
	}

	return nil, false
}

func (r *payloadQueue) get(tsn uint32) (*chunkPayloadData, bool) {
	return r.orderedPackets.search(tsn)
}

func (r *payloadQueue) popDuplicates() []uint32 {
	dups := r.dupTSN
	r.dupTSN = []uint32{}
	return dups
}

func (r *payloadQueue) getGapAckBlocks(cumulativeTSN uint32) (gapAckBlocks []gapAckBlock) {
	var b gapAckBlock

	if len(r.orderedPackets) == 0 {
		return []gapAckBlock{}
	}

	for i, p := range r.orderedPackets {
		if i == 0 {
			b.start = uint16(r.orderedPackets[0].tsn - cumulativeTSN)
			b.end = b.start
			continue
		}
		diff := uint16(p.tsn - cumulativeTSN)
		if b.end+1 == diff {
			b.end++
		} else {
			gapAckBlocks = append(gapAckBlocks, gapAckBlock{
				start: b.start,
				end:   b.end,
			})
			b.start = diff
			b.end = diff
		}
	}

	gapAckBlocks = append(gapAckBlocks, gapAckBlock{
		start: b.start,
		end:   b.end,
	})

	return gapAckBlocks
}
