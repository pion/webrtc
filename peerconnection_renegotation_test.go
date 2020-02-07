// +build !js

package webrtc

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/stretchr/testify/assert"
)

/*
*  Assert the following behaviors
* - We are able to call AddTrack after signaling
* - OnTrack is NOT called on the other side until after SetRemoteDescription
* - We are able to re-negotiate and AddTrack is properly called
 */
func TestPeerConnection_Renegotation_AddTrack(t *testing.T) {
	const (
		expectedTrackID    = "video"
		expectedTrackLabel = "pion"
	)

	api := NewAPI()
	// lim := test.TimeOut(time.Second * 30)
	// defer lim.Stop()

	// report := test.CheckRoutines(t)
	// defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	haveRenegotiated := &atomicBool{}
	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
		if !haveRenegotiated.get() {
			t.Fatal("OnTrack was called before renegotation")
		}
		onTrackFiredFunc()
	})

	haveConnected, haveConnectedFunc := context.WithCancel(context.Background())
	pcOffer.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateConnected {
			haveConnectedFunc()
		}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	<-haveConnected.Done()

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), expectedTrackID, expectedTrackLabel)
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)

	// Send 10 packets, OnTrack MUST not be fired
	for i := 0; i <= 10; i++ {
		assert.NoError(t, vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
		time.Sleep(20 * time.Millisecond)
	}

	haveRenegotiated.set(true)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	func() {
		for {
			select {
			case <-time.After(20 * time.Millisecond):
				assert.NoError(t, vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
			case <-onTrackFired.Done():
				return
			}
		}
	}()

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}
