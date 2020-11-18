// +build !js

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAPI(t *testing.T) {
	api := NewAPI()

	if api.settingEngine == nil {
		t.Error("Failed to init settings engine")
	}

	if api.mediaEngine == nil {
		t.Error("Failed to init media engine")
	}
}

func TestNewAPI_Options(t *testing.T) {
	s := SettingEngine{}
	s.DetachDataChannels()
	m := MediaEngine{}
	assert.NoError(t, m.RegisterDefaultCodecs())

	api := NewAPI(
		WithSettingEngine(s),
		WithMediaEngine(&m),
	)

	if !api.settingEngine.detach.DataChannels {
		t.Error("Failed to set settings engine")
	}

	if len(api.mediaEngine.audioCodecs) == 0 || len(api.mediaEngine.videoCodecs) == 0 {
		t.Error("Failed to set media engine")
	}
}
