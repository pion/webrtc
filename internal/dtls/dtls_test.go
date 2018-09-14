package dtls

import (
	"testing"
)

func TestNewState(t *testing.T) {
	dtls1 := NewState()
	defer dtls1.Close()
	dtls1.Start(true)

	dtls2 := NewState()
	defer dtls2.Close()
	dtls2.Start(false)
}
