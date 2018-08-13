package sdp

import (
	"strings"
	"strconv"
)

type TimeDescription struct {
	// t=<start-time> <stop-time>
	// https://tools.ietf.org/html/rfc4566#section-5.9
	Timing      Timing

	// r=<repeat interval> <active duration> <offsets from start-time>
	// https://tools.ietf.org/html/rfc4566#section-5.10
	RepeatTimes *RepeatTimes
}


type Timing struct {
	StartTime uint64
	StopTime  uint64
}

func (t *Timing) String() *string {
	output := strconv.FormatUint(t.StartTime, 10)
	output += ":" + strconv.FormatUint(t.StopTime, 10)
	return &output
}

type RepeatTimes struct {
	RepeatInterval int64
	ActiveDuration int64
	Offsets        []int64
}

func (r *RepeatTimes) String() *string {
	var fields []string
	fields = append(fields, strconv.FormatInt(r.RepeatInterval, 10))
	fields = append(fields, strconv.FormatInt(r.ActiveDuration, 10))
	for _, value := range r.Offsets {
		fields = append(fields, strconv.FormatInt(value, 10))
	}

	output := strings.Join(fields, " ")
	return &output
}