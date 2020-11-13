package webrtc

import (
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/vnet"
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
	offerVNet := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.4"},
	})
	// Add the network interface to the router
	assert.NoError(t, wan.AddNet(offerVNet))

	offerSettingEngine := SettingEngine{}
	offerSettingEngine.SetVNet(offerVNet)
	offerSettingEngine.SetICETimeouts(time.Second, time.Second, time.Millisecond*200)

	// Create a network interface for answerer
	answerVNet := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.5"},
	})
	// Add the network interface to the router
	assert.NoError(t, wan.AddNet(answerVNet))

	answerSettingEngine := SettingEngine{}
	answerSettingEngine.SetVNet(answerVNet)
	answerSettingEngine.SetICETimeouts(time.Second, time.Second, time.Millisecond*200)

	// Start the virtual network by calling Start() on the root router
	assert.NoError(t, wan.Start())

	offerPeerConnection, err := NewAPI(WithSettingEngine(offerSettingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerPeerConnection, err := NewAPI(WithSettingEngine(answerSettingEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	return offerPeerConnection, answerPeerConnection, wan
}
