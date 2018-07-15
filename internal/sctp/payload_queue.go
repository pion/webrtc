package sctp

import "sort"

type PayloadDataArray []*PayloadData

func (s PayloadDataArray) Search(tsn uint32) (*PayloadData, bool) {
	i := sort.Search(len(s), func(i int) bool {
		return s[i].TSN >= tsn
	})

	if i < len(s) && s[i].TSN == tsn {
		return s[i], true
	} else {
		return nil, false
	}
}

func (s PayloadDataArray) Sort() {
	sort.Slice(s, func(i, j int) bool { return s[i].TSN < s[j].TSN })
}

type PayloadQueue struct {
	orderedPackets PayloadDataArray
	dupTSN         []uint32
}

func (r *PayloadQueue) Push(p *PayloadData, cumulativeTSN uint32) {
	_, ok := r.orderedPackets.Search(p.TSN)

	// If the Data payload is already in our queue or older than our cumulativeTSN marker
	if ok || p.TSN <= cumulativeTSN {
		// Found the packet, log in dups
		r.dupTSN = append(r.dupTSN, p.TSN)
		return
	}

	r.orderedPackets = append(r.orderedPackets, p)
	r.orderedPackets.Sort()
}

func (r *PayloadQueue) Pop(tsn uint32) (*PayloadData, bool) {
	if len(r.orderedPackets) > 0 && tsn == r.orderedPackets[0].TSN {
		pd := r.orderedPackets[0]
		r.orderedPackets = r.orderedPackets[1:]
		return pd, true
	}

	return nil, false
}

func (r *PayloadQueue) PopDuplicates() []uint32 {
	dups := r.dupTSN
	r.dupTSN = []uint32{}
	return dups
}

func (r *PayloadQueue) GetGapAckBlocks(cumulativeTSN uint32) (gapAckBlocks []*GapAckBlock) {
	var b GapAckBlock

	for i, p := range r.orderedPackets {
		if i == 0 {
			b.start = uint16(r.orderedPackets[0].TSN - cumulativeTSN)
			b.end = b.start
			continue
		}
		diff := uint16(p.TSN - cumulativeTSN)
		if b.end+1 == diff {
			b.end++
		} else {
			gapAckBlocks = append(gapAckBlocks, &GapAckBlock{
				start: b.start,
				end:   b.end,
			})
			b.start = diff
			b.end = diff
		}
	}

	gapAckBlocks = append(gapAckBlocks, &GapAckBlock{
		start: b.start,
		end:   b.end,
	})

	return gapAckBlocks
}
