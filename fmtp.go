package webrtc

import (
	"strings"
)

type fmtp map[string]string

// parseFmtp parses fmtp string.
func parseFmtp(line string) fmtp {
	f := fmtp{}
	for _, p := range strings.Split(line, ";") {
		pp := strings.SplitN(strings.TrimSpace(p), "=", 2)
		key := strings.ToLower(pp[0])
		var value string
		if len(pp) > 1 {
			value = pp[1]
		}
		f[key] = value
	}
	return f
}

// fmtpConsist checks that two FMTP parameters are not inconsistent.
func fmtpConsist(a, b fmtp) bool {
	for k, v := range a {
		if vb, ok := b[k]; ok && !strings.EqualFold(vb, v) {
			return false
		}
	}
	for k, v := range b {
		if va, ok := a[k]; ok && !strings.EqualFold(va, v) {
			return false
		}
	}
	return true
}
