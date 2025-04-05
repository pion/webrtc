// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAPI(t *testing.T) {
	api := NewAPI()
	assert.NotNil(t, api.settingEngine, "failed to init settings engine")
	assert.NotNil(t, api.mediaEngine, "failed to init media engine")
	assert.NotNil(t, api.interceptorRegistry, "failed to init interceptor registry")
}

func TestNewAPI_Options(t *testing.T) {
	s := SettingEngine{}
	s.DetachDataChannels()

	api := NewAPI(
		WithSettingEngine(s),
	)

	assert.True(t, api.settingEngine.detach.DataChannels, "failed to set settings engine")
	assert.NotEmpty(t, api.mediaEngine.audioCodecs, "failed to set audio codecs")
	assert.NotEmpty(t, api.mediaEngine.videoCodecs, "failed to set video codecs")
}

func TestNewAPI_OptionsDefaultize(t *testing.T) {
	api := NewAPI(
		WithMediaEngine(nil),
		WithInterceptorRegistry(nil),
	)

	assert.NotNil(t, api.settingEngine)
	assert.NotNil(t, api.mediaEngine)
	assert.NotNil(t, api.interceptorRegistry)
}
