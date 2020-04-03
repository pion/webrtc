// +build !js

package webrtc

import (
	"context"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v2/internal/util"
	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/stretchr/testify/assert"
)

func sendVideoUntilDone(done <-chan struct{}, t *testing.T, tracks []*Track) {
	for {
		select {
		case <-time.After(20 * time.Millisecond):
			for _, track := range tracks {
				assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
			}
		case <-done:
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

	sendVideoUntilDone(onTrackFired.Done(), t, []*Track{vp8Track})

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

// Assert that adding tracks across multiple renegotations performs as expected
func TestPeerConnection_Renegotation_AddTrack_Multiple(t *testing.T) {
	addTrackWithLabel := func(trackName string, pcOffer, pcAnswer *PeerConnection) *Track {
		_, err := pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
		assert.NoError(t, err)

		track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), trackName, trackName)
		assert.NoError(t, err)

		_, err = pcOffer.AddTrack(track)
		assert.NoError(t, err)

		return track
	}

	trackNames := []string{util.RandSeq(trackDefaultIDLength), util.RandSeq(trackDefaultIDLength), util.RandSeq(trackDefaultIDLength)}
	outboundTracks := []*Track{}
	onTrackCount := map[string]int{}
	onTrackCountMu := sync.Mutex{}
	onTrackChan := make(chan struct{}, 1)

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

	pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
		onTrackChan <- struct{}{}

		onTrackCountMu.Lock()
		defer onTrackCountMu.Unlock()
		onTrackCount[track.Label()]++
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	for i := range trackNames {
		outboundTracks = append(outboundTracks, addTrackWithLabel(trackNames[i], pcOffer, pcAnswer))
		assert.NoError(t, signalPair(pcOffer, pcAnswer))
		sendVideoUntilDone(onTrackChan, t, outboundTracks)
	}

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())

	onTrackCountMu.Lock()
	defer onTrackCountMu.Unlock()
	assert.Equal(t, onTrackCount[trackNames[0]], 1)
	assert.Equal(t, onTrackCount[trackNames[1]], 1)
	assert.Equal(t, onTrackCount[trackNames[2]], 1)
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
	sendVideoUntilDone(onTrackFired.Done(), t, []*Track{vp8Track})

	assert.NoError(t, pcOffer.RemoveTrack(rtpSender))
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-trackClosed.Done()
	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
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

	assert.NoError(t, signalPair(pcFirstOfferer, pcSecondOfferer))

	// Add a new Track to the second offerer
	// This asserts that it will match the ordering of the last RemoteDescription, but then also add new Transceivers to the end
	_, err = pcFirstOfferer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := pcSecondOfferer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
	assert.NoError(t, err)

	_, err = pcSecondOfferer.AddTrack(vp8Track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcSecondOfferer, pcFirstOfferer))
	sendVideoUntilDone(onTrackFired.Done(), t, []*Track{vp8Track})

	assert.NoError(t, pcFirstOfferer.Close())
	assert.NoError(t, pcSecondOfferer.Close())
}
