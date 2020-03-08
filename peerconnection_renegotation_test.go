// +build !js

package webrtc

import (
	"context"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/stretchr/testify/assert"
)

func sendVideoUntilDone(c context.Context, t *testing.T, track *Track) {
	for {
		select {
		case <-time.After(20 * time.Millisecond):
			assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
		case <-c.Done():
			return
		}
	}
}

/*
*  Assert the following behaviors
* - We are able to call AddTrack after signaling
* - OnTrack is NOT called on the other side until after SetRemoteDescription
* - We are able to re-negotiate and AddTrack is properly called
 */
func TestPeerConnection_Renegotation_AddTrack(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

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

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
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

	sendVideoUntilDone(onTrackFired, t, vp8Track)

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

func TestPeerConnection_Renegotation_RemoveTrack(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
	assert.NoError(t, err)

	rtpSender, err := pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	trackClosed, trackClosedFunc := context.WithCancel(context.Background())

	pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
		onTrackFiredFunc()

		for {
			if _, err := track.ReadRTP(); err == io.EOF {
				trackClosedFunc()
				return
			}
		}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	sendVideoUntilDone(onTrackFired, t, vp8Track)

	assert.NoError(t, pcOffer.RemoveTrack(rtpSender))
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-trackClosed.Done()
	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

// When creating an offer the first media section MUST be SCTP
// This is to make renegotation easier, instead of having to remember where it
// is was interleaved
func TestPeerConnection_Renegotation_ApplicationFirst(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	addTransceiverAndAssert := func() {
		_, err = pc.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		offer, err := pc.CreateOffer(nil)
		assert.NoError(t, err)

		assert.Equal(t, offer.parsed.MediaDescriptions[0].MediaName.Media, "application")
	}

	addTransceiverAndAssert()
	addTransceiverAndAssert()

	assert.NoError(t, pc.Close())
}

func TestPeerConnection_RoleSwitch(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcFirstOfferer, pcSecondOfferer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcFirstOfferer.OnTrack(func(track *Track, r *RTPReceiver) {
		onTrackFiredFunc()
	})

	connected, connectedFunc := context.WithCancel(context.Background())
	pcFirstOfferer.OnConnectionStateChange(func(c PeerConnectionState) {
		if c == PeerConnectionStateConnected {
			connectedFunc()
		}
	})

	assert.NoError(t, signalPair(pcFirstOfferer, pcSecondOfferer))
	<-connected.Done()

	// Add a new Track to the second offerer
	// This asserts that it will match the ordering of the last RemoteDescription, but then also add new Transceivers to the end
	_, err = pcFirstOfferer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := pcSecondOfferer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
	assert.NoError(t, err)

	_, err = pcSecondOfferer.AddTrack(vp8Track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcSecondOfferer, pcFirstOfferer))
	sendVideoUntilDone(onTrackFired, t, vp8Track)

	assert.NoError(t, pcFirstOfferer.Close())
	assert.NoError(t, pcSecondOfferer.Close())
}
