// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/transport/v3/vnet"
	"github.com/stretchr/testify/assert"
)

func createVNetPair(t *testing.T) (*PeerConnection, *PeerConnection, *vnet.Router) {
	// Create a root router
	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	assert.NoError(t, err)

	// Create a network interface for offerer
	offerVNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.4"},
	})
	assert.NoError(t, err)

	// Add the network interface to the router
	assert.NoError(t, wan.AddNet(offerVNet))

	offerSettingEngine := SettingEngine{}
	offerSettingEngine.SetNet(offerVNet)
	offerSettingEngine.SetICETimeouts(time.Second, time.Second, time.Millisecond*200)

	// Create a network interface for answerer
	answerVNet, err := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.5"},
	})
	assert.NoError(t, err)

	// Add the network interface to the router
	assert.NoError(t, wan.AddNet(answerVNet))

	answerSettingEngine := SettingEngine{}
	answerSettingEngine.SetNet(answerVNet)
	answerSettingEngine.SetICETimeouts(time.Second, time.Second, time.Millisecond*200)

	// Start the virtual network by calling Start() on the root router
	assert.NoError(t, wan.Start())

	offerInterceptorRegistry := &interceptor.Registry{}
	offerPeerConnection, err := NewAPI(WithSettingEngine(offerSettingEngine), WithInterceptorRegistry(offerInterceptorRegistry)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerInterceptorRegistry := &interceptor.Registry{}
	answerPeerConnection, err := NewAPI(WithSettingEngine(answerSettingEngine), WithInterceptorRegistry(answerInterceptorRegistry)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	return offerPeerConnection, answerPeerConnection, wan
}
