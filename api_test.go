package webrtc

import (
	"testing"
)

func TestNewAPI(t *testing.T) {
	api := NewAPI()

	if api.mediaEngine == nil {
		t.Error("Failed to init media engine")
	}
}

func TestNewAPI_Options(t *testing.T) {
	m := MediaEngine{}
	m.RegisterDefaultCodecs()

	api := NewAPI(
		WithMediaEngine(m),
	)

	if len(api.mediaEngine.codecs) == 0 {
		t.Error("Failed to set media engine")
	}
}
