package webrtc

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Renegotiate but don't change any state of the PeerConnection
func TestPeerConnection_Renegotiate_Basic(t *testing.T) {
	var peerConnected sync.WaitGroup
	peerConnected.Add(2)

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	connectionStateHandler := func(s PeerConnectionState) {
		if s == PeerConnectionStateConnected {
			peerConnected.Done()
		}
	}
	pcOffer.OnConnectionStateChange(connectionStateHandler)
	pcAnswer.OnConnectionStateChange(connectionStateHandler)

	// Negotiate, block until connected
	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	peerConnected.Wait()

	// Re-negotiate, we haven't changed any state
	pcOffer.OnConnectionStateChange(func(PeerConnectionState) {})
	pcAnswer.OnConnectionStateChange(func(PeerConnectionState) {})

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	assert.NoError(t, pcOffer.SetRemoteDescription(answer))
}
