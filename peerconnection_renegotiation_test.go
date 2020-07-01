// +build !js

package webrtc

import (
	"context"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func sdpMidHasSsrc(offer SessionDescription, mid string, ssrc uint32) bool {
	for _, media := range offer.parsed.MediaDescriptions {
		cmid, ok := media.Attribute("mid")
		if !ok {
			continue
		}
		if cmid != mid {
			continue
		}
		cssrc, ok := media.Attribute("ssrc")
		if !ok {
			continue
		}
		parts := strings.Split(cssrc, " ")

		ssrcInt64, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			continue
		}

		if uint32(ssrcInt64) == ssrc {
			return true
		}
	}
	return false
}

/*
*  Assert the following behaviors
* - We are able to call AddTrack after signaling
* - OnTrack is NOT called on the other side until after SetRemoteDescription
* - We are able to re-negotiate and AddTrack is properly called
 */
func TestPeerConnection_Renegotiation_AddTrack(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	haveRenegotiated := &atomicBool{}
	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
		if !haveRenegotiated.get() {
			t.Fatal("OnTrack was called before renegotiation")
		}
		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
	assert.NoError(t, err)

	sender, err := pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)

	// Send 10 packets, OnTrack MUST not be fired
	for i := 0; i <= 10; i++ {
		assert.NoError(t, vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
		time.Sleep(20 * time.Millisecond)
	}

	haveRenegotiated.set(true)
	assert.False(t, sender.isNegotiated())
	offer, err := pcOffer.CreateOffer(nil)
	assert.True(t, sender.isNegotiated())
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcAnswer.SetLocalDescription(answer))

	pcOffer.ops.Done()
	assert.Equal(t, 0, len(vp8Track.activeSenders))

	assert.NoError(t, pcOffer.SetRemoteDescription(answer))

	pcOffer.ops.Done()
	assert.Equal(t, 1, len(vp8Track.activeSenders))

	sendVideoUntilDone(onTrackFired.Done(), t, []*Track{vp8Track})

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

// Assert that adding tracks across multiple renegotiations performs as expected
func TestPeerConnection_Renegotiation_AddTrack_Multiple(t *testing.T) {
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
	onTrackChan := make(chan struct{}, 1)

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
		onTrackCount[track.Label()]++
		onTrackChan <- struct{}{}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	for i := range trackNames {
		outboundTracks = append(outboundTracks, addTrackWithLabel(trackNames[i], pcOffer, pcAnswer))
		assert.NoError(t, signalPair(pcOffer, pcAnswer))
		sendVideoUntilDone(onTrackChan, t, outboundTracks)
	}

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())

	assert.Equal(t, onTrackCount[trackNames[0]], 1)
	assert.Equal(t, onTrackCount[trackNames[1]], 1)
	assert.Equal(t, onTrackCount[trackNames[2]], 1)
}

// Assert that renegotiation triggers OnTrack() with correct ID and label from
// remote side, even when a transceiver was added before the actual track data
// was received. This happens when we add a transceiver on the server, create
// an offer on the server and the browser's answer contains the same SSRC, but
// a track hasn't been added on the browser side yet. The browser can add a
// track later and renegotiate, and track ID and label will be set by the time
// first packets are received.
func TestPeerConnection_Renegotiation_AddTrack_Rename(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	haveRenegotiated := &atomicBool{}
	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	var atomicRemoteTrack atomic.Value
	pcOffer.OnTrack(func(track *Track, r *RTPReceiver) {
		if !haveRenegotiated.get() {
			t.Fatal("OnTrack was called before renegotiation")
		}
		onTrackFiredFunc()
		atomicRemoteTrack.Store(track)
	})

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)
	vp8Track, err := pcAnswer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo1", "bar1")
	assert.NoError(t, err)
	_, err = pcAnswer.AddTrack(vp8Track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	vp8Track.id = "foo2"
	vp8Track.label = "bar2"

	haveRenegotiated.set(true)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	sendVideoUntilDone(onTrackFired.Done(), t, []*Track{vp8Track})

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())

	remoteTrack, ok := atomicRemoteTrack.Load().(*Track)
	require.True(t, ok)
	require.NotNil(t, remoteTrack)
	assert.Equal(t, vp8Track.SSRC(), remoteTrack.SSRC())
	assert.Equal(t, "foo2", remoteTrack.ID())
	assert.Equal(t, "bar2", remoteTrack.Label())
}

// TestPeerConnection_Transceiver_Mid tests that we'll provide the same
// transceiver for a media id on successive offer/answer
func TestPeerConnection_Transceiver_Mid(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track1, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion1")
	require.NoError(t, err)

	sender1, err := pcOffer.AddTrack(track1)
	require.NoError(t, err)

	track2, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
	require.NoError(t, err)

	_, err = pcOffer.AddTrack(track2)
	require.NoError(t, err)

	// this will create the initial offer using generateUnmatchedSDP
	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringComplete

	assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)

	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringComplete

	// apply answer so we'll test generateMatchedSDP
	assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	// Must have 3 media descriptions (2 video and 1 datachannel)
	assert.Equal(t, len(offer.parsed.MediaDescriptions), 3)

	assert.True(t, sdpMidHasSsrc(offer, "0", track1.SSRC()), "Expected mid %q with ssrc %d, offer.SDP: %s", "0", track1.SSRC(), offer.SDP)

	// Remove first track, must keep same number of media
	// descriptions and same track ssrc for mid 1 as previous
	err = pcOffer.RemoveTrack(sender1)
	assert.NoError(t, err)

	offer, err = pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	assert.Equal(t, len(offer.parsed.MediaDescriptions), 3)

	assert.True(t, sdpMidHasSsrc(offer, "1", track2.SSRC()), "Expected mid %q with ssrc %d, offer.SDP: %s", "1", track2.SSRC(), offer.SDP)

	answer, err = pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	// apply answer so we'll test generateMatchedSDP
	assert.NoError(t, pcOffer.SetRemoteDescription(answer))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	track3, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion3")
	require.NoError(t, err)

	_, err = pcOffer.AddTrack(track3)
	require.NoError(t, err)

	offer, err = pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	// We reuse the existing non-sending transceiver
	assert.Equal(t, len(offer.parsed.MediaDescriptions), 3)

	assert.True(t, sdpMidHasSsrc(offer, "0", track3.SSRC()), "Expected mid %q with ssrc %d, offer.sdp: %s", "0", track3.SSRC(), offer.SDP)
	assert.True(t, sdpMidHasSsrc(offer, "1", track2.SSRC()), "Expected mid %q with ssrc %d, offer.sdp: %s", "1", track2.SSRC(), offer.SDP)

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

func TestPeerConnection_Renegotiation_CodecChange(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track1, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, 123, "video1", "pion1")
	require.NoError(t, err)

	track2, err := pcOffer.NewTrack(DefaultPayloadTypeVP9, 456, "video2", "pion2")
	require.NoError(t, err)

	sender1, err := pcOffer.AddTrack(track1)
	require.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	require.NoError(t, err)

	tracksCh := make(chan *Track)
	tracksClosed := make(chan struct{})
	pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
		tracksCh <- track
		for {
			if _, readErr := track.ReadRTP(); readErr == io.EOF {
				tracksClosed <- struct{}{}
				return
			}
		}
	})

	err = signalPair(pcOffer, pcAnswer)
	require.NoError(t, err)

	transceivers := pcOffer.GetTransceivers()
	require.Equal(t, 1, len(transceivers))
	require.Equal(t, "0", transceivers[0].Mid())

	transceivers = pcAnswer.GetTransceivers()
	require.Equal(t, 1, len(transceivers))
	require.Equal(t, "0", transceivers[0].Mid())

	ctx, cancel := context.WithCancel(context.Background())
	go sendVideoUntilDone(ctx.Done(), t, []*Track{track1})

	remoteTrack1 := <-tracksCh
	cancel()

	assert.Equal(t, uint32(123), remoteTrack1.SSRC())
	assert.Equal(t, "video1", remoteTrack1.ID())
	assert.Equal(t, "pion1", remoteTrack1.Label())

	err = pcOffer.RemoveTrack(sender1)
	require.NoError(t, err)

	sender2, err := pcOffer.AddTrack(track2)
	require.NoError(t, err)

	err = signalPair(pcOffer, pcAnswer)
	require.NoError(t, err)
	<-tracksClosed

	transceivers = pcOffer.GetTransceivers()
	require.Equal(t, 1, len(transceivers))
	require.Equal(t, "0", transceivers[0].Mid())

	transceivers = pcAnswer.GetTransceivers()
	require.Equal(t, 1, len(transceivers))
	require.Equal(t, "0", transceivers[0].Mid())

	ctx, cancel = context.WithCancel(context.Background())
	go sendVideoUntilDone(ctx.Done(), t, []*Track{track2})

	remoteTrack2 := <-tracksCh
	cancel()

	err = pcOffer.RemoveTrack(sender2)
	require.NoError(t, err)

	err = signalPair(pcOffer, pcAnswer)
	require.NoError(t, err)
	<-tracksClosed

	assert.Equal(t, uint32(456), remoteTrack2.SSRC())
	assert.Equal(t, "video2", remoteTrack2.ID())
	assert.Equal(t, "pion2", remoteTrack2.Label())

	require.NoError(t, pcOffer.Close())
	require.NoError(t, pcAnswer.Close())
}

func TestPeerConnection_Renegotiation_RemoveTrack(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
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
	pcFirstOfferer, pcSecondOfferer, err := api.newPair(Configuration{})
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

// Assert that renegotiation doesn't attempt to gather ICE twice
// Before we would attempt to gather multiple times and would put
// the PeerConnection into a broken state
func TestPeerConnection_Renegotiation_Trickle(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	settingEngine := SettingEngine{}

	api := NewAPI(WithSettingEngine(settingEngine))
	api.mediaEngine.RegisterDefaultCodecs()

	// Invalid STUN server on purpose, will stop ICE Gathering from completing in time
	pcOffer, pcAnswer, err := api.newPair(Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:127.0.0.1:5000"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	pcOffer.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			assert.NoError(t, pcAnswer.AddICECandidate(c.ToJSON()))
		} else {
			wg.Done()
		}
	})
	pcAnswer.OnICECandidate(func(c *ICECandidate) {
		if c != nil {
			assert.NoError(t, pcOffer.AddICECandidate(c.ToJSON()))
		} else {
			wg.Done()
		}
	})

	negotiate := func() {
		offer, err := pcOffer.CreateOffer(nil)
		assert.NoError(t, err)

		assert.NoError(t, pcOffer.SetLocalDescription(offer))
		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		assert.NoError(t, pcAnswer.SetLocalDescription(answer))
		assert.NoError(t, pcOffer.SetRemoteDescription(answer))
	}
	negotiate()
	negotiate()

	pcOffer.ops.Done()
	pcAnswer.ops.Done()
	wg.Wait()

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

func TestPeerConnection_Renegotiation_SetLocalDescription(t *testing.T) {
	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcOffer.OnTrack(func(track *Track, r *RTPReceiver) {
		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	localTrack, err := pcAnswer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
	assert.NoError(t, err)

	sender, err := pcAnswer.AddTrack(localTrack)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
	assert.False(t, sender.isNegotiated())
	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.True(t, sender.isNegotiated())

	pcAnswer.ops.Done()
	assert.Equal(t, 0, len(localTrack.activeSenders))

	assert.NoError(t, pcAnswer.SetLocalDescription(answer))

	pcAnswer.ops.Done()
	assert.Equal(t, 1, len(localTrack.activeSenders))

	assert.NoError(t, pcOffer.SetRemoteDescription(answer))

	sendVideoUntilDone(onTrackFired.Done(), t, []*Track{localTrack})

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

// Issue #346, don't start the SCTP Subsystem if the RemoteDescription doesn't contain one
// Before we would always start it, and re-negotations would fail because SCTP was in flight
func TestPeerConnection_Renegotiation_NoApplication(t *testing.T) {
	signalPairExcludeDataChannel := func(pcOffer, pcAnswer *PeerConnection) {
		offer, err := pcOffer.CreateOffer(nil)
		assert.NoError(t, err)
		offerGatheringComplete := GatheringCompletePromise(pcOffer)
		assert.NoError(t, pcOffer.SetLocalDescription(offer))
		<-offerGatheringComplete

		offer = *pcOffer.LocalDescription()
		offer.SDP = strings.Split(offer.SDP, "m=application")[0]
		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		answerGatheringComplete := GatheringCompletePromise(pcAnswer)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))
		<-answerGatheringComplete

		assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))
	}

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionSendrecv})
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionSendrecv})
	assert.NoError(t, err)

	// Setting SCTPTransport to nil ensures any interaction with it will cause a segafault
	pcAnswer.sctpTransport = nil

	signalPairExcludeDataChannel(pcOffer, pcAnswer)
	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	signalPairExcludeDataChannel(pcOffer, pcAnswer)
	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}
