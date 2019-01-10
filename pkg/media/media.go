package media

import "time"

// RTCSample contains media, and the amount of samples in it
type RTCSample struct {
	Data     []byte
	Duration time.Duration
}
