// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"net"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/transport/v4/test"
	"github.com/stretchr/testify/assert"
)

func TestNewICEUniversalUDPMux(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	assert.NoError(t, err)

	udpMux := NewICEUniversalUDPMux(nil, udpConn, 30*time.Second)
	assert.NotNil(t, udpMux)

	// The universal mux embeds ice.UDPMuxDefault, so it also satisfies the plain
	// ice.UDPMux interface. This allows a single mux to be passed to both
	// SetICEUDPMux and SetICEUDPMuxSrflx.
	var _ ice.UDPMux = udpMux

	settingEngine := SettingEngine{}
	settingEngine.SetICEUDPMux(udpMux)
	settingEngine.SetICEUDPMuxSrflx(udpMux)
	assert.Equal(t, ice.UDPMux(udpMux), settingEngine.iceUDPMux)
	assert.Equal(t, udpMux, settingEngine.iceUDPMuxSrflx)

	assert.NoError(t, udpMux.Close())
}
