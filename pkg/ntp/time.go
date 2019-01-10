package ntp

import (
	"errors"
	"time"
)

var (
	epoch = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	// CurrentEra is the most recent NTP era (RFC 5905 Section 6). It is used
	// by conversions between Time64 and time.Time and may be overridden for
	// testing or parsing historical timestamps.
	CurrentEra = era(time.Now())
)

func era(t time.Time) int32 {
	s := t.Sub(epoch) / time.Second
	return int32(s >> 32)
}

// Time64 is a 64-bit unsigned fixed-point number (Q32.32) which encodes
// the number of seconds since 0h UTC on 1 January 1900, with a precision
// of about 200 picoseconds.
//
// The field will overflow in 2036 and every 136 years thereafter. For purposes
// of conversion to time.Time, This implementation assumes that the time is
// encoded in the most recent NTP era (RFC 5905 Section 6).
type Time64 uint64

// Duration returns the amount of time since the epoch represented by this timestamp.
func (t Time64) Duration() time.Duration {
	sec := time.Duration(t>>32) * time.Second
	frac := time.Duration(t&0xffffffff) * time.Second >> 32
	return sec + frac
}

// Time returns the Go Time represented by this timestamp.
//
// Conversions are made relative to the most recent NTP era's epoch (RFC 5905 Section 6).
// The field overflows every 136 years, triggering a new era. This function may
// return different results for the same timestamp around the start of a new era.
func (t Time64) Time() time.Time {
	eraOffset := time.Duration(CurrentEra) << 32 * time.Second
	return epoch.Add(t.Duration()).Add(eraOffset)
}

// Time32 is an abbreviated timestamp formed using the middle 32 bits of a Time64
// timestamp. It's a 32-bit unsigned fixed-point number (Q16.16) representing the
// number of seconds since the NTP epoch. The high 16 bits are the integer part
// and the low 16 bits are the fractional part.
//
// Because the timestamp overflows after around 18 hours it's only useful for
// encoding relative durations between timestamps.
type Time32 uint32

// NewTime32 converts a time.Duration into a Time32
//
// Negative durations and durations greater than 65536s are invalid and will
// produce an error.
func NewTime32(d time.Duration) (Time32, error) {
	if d < 0 {
		return Time32(0), errors.New("duration must be positive")
	}
	if d > (1 << 16) {
		return Time32(0), errors.New("duration must be less than d > (1<<16)")
	}
	sec := d / time.Second
	frac := (d - sec*time.Second) << 16
	frac = (frac + time.Second - 1) / time.Second
	return Time32(sec<<32 | frac), nil
}

// Duration returns the amount of time since the epoch represented by this timestamp.
func (t Time32) Duration() time.Duration {
	t64 := uint64(t)
	sec := (t64 >> 16) * uint64(time.Second)
	frac := (t64 & 0xffff) * uint64(time.Second) >> 16
	return time.Duration(sec + frac)
}
