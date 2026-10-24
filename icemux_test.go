// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc_test

import (
	"net"

	"github.com/pion/ice/v5"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v5"
)

// Public mux signatures stay independent of the ICE module version.
var (
	_ webrtc.ICETCPMux = (*ice.TCPMuxDefault)(nil)
	_ webrtc.ICEUDPMux = (*ice.UDPMuxDefault)(nil)
	_ webrtc.ICEUDPMux = (*ice.UniversalUDPMuxDefault)(nil)

	_ func(logging.LeveledLogger, net.Listener, int) webrtc.ICETCPMux = webrtc.NewICETCPMux
	_ func(logging.LeveledLogger, net.PacketConn) webrtc.ICEUDPMux    = webrtc.NewICEUDPMux
	_ func(*webrtc.SettingEngine, webrtc.ICETCPMux)                   = (*webrtc.SettingEngine).SetICETCPMux
	_ func(*webrtc.SettingEngine, webrtc.ICEUDPMux)                   = (*webrtc.SettingEngine).SetICEUDPMux
)
