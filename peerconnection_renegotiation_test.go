// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/transport/v2/test"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sendVideoUntilDone(done <-chan struct{}, t *testing.T, tracks []*TrackLocalStaticSample) {
	for {
		select {
		case <-time.After(20 * time.Millisecond):
			for _, track := range tracks {
				assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}))
			}
		case <-done:
			return
		}
	}
}

func sdpMidHasSsrc(offer SessionDescription, mid string, ssrc SSRC) bool {
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

		if uint32(ssrcInt64) == uint32(ssrc) {
			return true
		}
	}
	return false
}

func TestPeerConnection_Renegotiation_AddRecvonlyTransceiver(t *testing.T) {
	type testCase struct {
		name          string
		answererSends bool
	}

	testCases := []testCase{
		// Assert the following behaviors:
		// - Offerer can add a recvonly transceiver
		// - During negotiation, answerer peer adds an inactive (or sendonly) transceiver
		// - Offerer can add a track
		// - Answerer can receive the RTP packets.
		{"add recvonly, then receive from answerer", false},
		// Assert the following behaviors:
		// - Offerer can add a recvonly transceiver
		// - During negotiation, answerer peer adds an inactive (or sendonly) transceiver
		// - Answerer can add a track to the existing sendonly transceiver
		// - Offerer can receive the RTP packets.
		{"add recvonly, then send to answerer", true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			lim := test.TimeOut(time.Second * 30)
			defer lim.Stop()

			report := test.CheckRoutines(t)
			defer report()

			pcOffer, pcAnswer, err := newPair()
			if err != nil {
				t.Fatal(err)
			}

			_, err = pcOffer.AddTransceiverFromKind(
				RTPCodecTypeVideo,
				RTPTransceiverInit{
					Direction: RTPTransceiverDirectionRecvonly,
				},
			)
			assert.NoError(t, err)

			assert.NoError(t, signalPair(pcOffer, pcAnswer))

			localTrack, err := NewTrackLocalStaticSample(
				RTPCodecCapability{MimeType: "video/VP8"}, "track-one", "stream-one",
			)
			require.NoError(t, err)

			if tc.answererSends {
				_, err = pcAnswer.AddTrack(localTrack)
			} else {
				_, err = pcOffer.AddTrack(localTrack)
			}

			require.NoError(t, err)

			onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())

			if tc.answererSends {
				pcOffer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
					onTrackFiredFunc()
				})
				assert.NoError(t, signalPair(pcAnswer, pcOffer))
			} else {
				pcAnswer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
					onTrackFiredFunc()
				})
				assert.NoError(t, signalPair(pcOffer, pcAnswer))
			}

			sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{localTrack})

			closePairNow(t, pcOffer, pcAnswer)
		})
	}
}

/*
*  Assert the following behaviors
* - We are able to call AddTrack after signaling
* - OnTrack is NOT called on the other side until after SetRemoteDescription
* - We are able to re-negotiate and AddTrack is properly called
 */
func TestPeerConnection_Renegotiation_AddTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	haveRenegotiated := &atomicBool{}
	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcAnswer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		if !haveRenegotiated.get() {
			t.Fatal("OnTrack was called before renegotiation")
		}
		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
	assert.NoError(t, err)

	sender, err := pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)

	// Send 10 packets, OnTrack MUST not be fired
	for i := 0; i <= 10; i++ {
		assert.NoError(t, vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}))
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
	assert.Equal(t, 0, len(vp8Track.rtpTrack.bindings))

	assert.NoError(t, pcOffer.SetRemoteDescription(answer))

	pcOffer.ops.Done()
	assert.Equal(t, 1, len(vp8Track.rtpTrack.bindings))

	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{vp8Track})

	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that adding tracks across multiple renegotiations performs as expected
func TestPeerConnection_Renegotiation_AddTrack_Multiple(t *testing.T) {
	addTrackWithLabel := func(trackID string, pcOffer, pcAnswer *PeerConnection) *TrackLocalStaticSample {
		_, err := pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
		assert.NoError(t, err)

		track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, trackID, trackID)
		assert.NoError(t, err)

		_, err = pcOffer.AddTrack(track)
		assert.NoError(t, err)

		return track
	}

	trackIDs := []string{util.MathRandAlpha(16), util.MathRandAlpha(16), util.MathRandAlpha(16)}
	outboundTracks := []*TrackLocalStaticSample{}
	onTrackCount := map[string]int{}
	onTrackChan := make(chan struct{}, 1)

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	pcAnswer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		onTrackCount[track.ID()]++
		onTrackChan <- struct{}{}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	for i := range trackIDs {
		outboundTracks = append(outboundTracks, addTrackWithLabel(trackIDs[i], pcOffer, pcAnswer))
		assert.NoError(t, signalPair(pcOffer, pcAnswer))
		sendVideoUntilDone(onTrackChan, t, outboundTracks)
	}

	closePairNow(t, pcOffer, pcAnswer)

	assert.Equal(t, onTrackCount[trackIDs[0]], 1)
	assert.Equal(t, onTrackCount[trackIDs[1]], 1)
	assert.Equal(t, onTrackCount[trackIDs[2]], 1)
}

// Assert that renegotiation triggers OnTrack() with correct ID and label from
// remote side, even when a transceiver was added before the actual track data
// was received. This happens when we add a transceiver on the server, create
// an offer on the server and the browser's answer contains the same SSRC, but
// a track hasn't been added on the browser side yet. The browser can add a
// track later and renegotiate, and track ID and label will be set by the time
// first packets are received.
func TestPeerConnection_Renegotiation_AddTrack_Rename(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	haveRenegotiated := &atomicBool{}
	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	var atomicRemoteTrack atomic.Value
	pcOffer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		if !haveRenegotiated.get() {
			t.Fatal("OnTrack was called before renegotiation")
		}
		onTrackFiredFunc()
		atomicRemoteTrack.Store(track)
	})

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)
	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo1", "bar1")
	assert.NoError(t, err)
	_, err = pcAnswer.AddTrack(vp8Track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	vp8Track.rtpTrack.id = "foo2"
	vp8Track.rtpTrack.streamID = "bar2"

	haveRenegotiated.set(true)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{vp8Track})

	closePairNow(t, pcOffer, pcAnswer)

	remoteTrack, ok := atomicRemoteTrack.Load().(*TrackRemote)
	require.True(t, ok)
	require.NotNil(t, remoteTrack)
	assert.Equal(t, "foo2", remoteTrack.ID())
	assert.Equal(t, "bar2", remoteTrack.StreamID())
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

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion1")
	require.NoError(t, err)

	sender1, err := pcOffer.AddTrack(track1)
	require.NoError(t, err)

	track2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
	require.NoError(t, err)

	sender2, err := pcOffer.AddTrack(track2)
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

	// Must have 3 media descriptions (2 video channels)
	assert.Equal(t, len(offer.parsed.MediaDescriptions), 2)

	assert.True(t, sdpMidHasSsrc(offer, "0", sender1.trackEncodings[0].ssrc), "Expected mid %q with ssrc %d, offer.SDP: %s", "0", sender1.trackEncodings[0].ssrc, offer.SDP)

	// Remove first track, must keep same number of media
	// descriptions and same track ssrc for mid 1 as previous
	assert.NoError(t, pcOffer.RemoveTrack(sender1))

	offer, err = pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))

	assert.Equal(t, len(offer.parsed.MediaDescriptions), 2)

	assert.True(t, sdpMidHasSsrc(offer, "1", sender2.trackEncodings[0].ssrc), "Expected mid %q with ssrc %d, offer.SDP: %s", "1", sender2.trackEncodings[0].ssrc, offer.SDP)

	_, err = pcAnswer.CreateAnswer(nil)
	assert.Equal(t, err, &rtcerr.InvalidStateError{Err: ErrIncorrectSignalingState})

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
	answer, err = pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetRemoteDescription(answer))

	track3, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion3")
	require.NoError(t, err)

	sender3, err := pcOffer.AddTrack(track3)
	require.NoError(t, err)

	offer, err = pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	// We reuse the existing non-sending transceiver
	assert.Equal(t, len(offer.parsed.MediaDescriptions), 2)

	assert.True(t, sdpMidHasSsrc(offer, "0", sender3.trackEncodings[0].ssrc), "Expected mid %q with ssrc %d, offer.sdp: %s", "0", sender3.trackEncodings[0].ssrc, offer.SDP)
	assert.True(t, sdpMidHasSsrc(offer, "1", sender2.trackEncodings[0].ssrc), "Expected mid %q with ssrc %d, offer.sdp: %s", "1", sender2.trackEncodings[0].ssrc, offer.SDP)

	closePairNow(t, pcOffer, pcAnswer)
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

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video1", "pion1")
	require.NoError(t, err)

	track2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video2", "pion2")
	require.NoError(t, err)

	sender1, err := pcOffer.AddTrack(track1)
	require.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	require.NoError(t, err)

	tracksCh := make(chan *TrackRemote)
	tracksClosed := make(chan struct{})
	pcAnswer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		tracksCh <- track
		for {
			if _, _, readErr := track.ReadRTP(); errors.Is(readErr, io.EOF) {
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
	go sendVideoUntilDone(ctx.Done(), t, []*TrackLocalStaticSample{track1})

	remoteTrack1 := <-tracksCh
	cancel()

	assert.Equal(t, "video1", remoteTrack1.ID())
	assert.Equal(t, "pion1", remoteTrack1.StreamID())

	require.NoError(t, pcOffer.RemoveTrack(sender1))

	require.NoError(t, signalPair(pcOffer, pcAnswer))
	<-tracksClosed

	sender2, err := pcOffer.AddTrack(track2)
	require.NoError(t, err)
	require.NoError(t, signalPair(pcOffer, pcAnswer))
	transceivers = pcOffer.GetTransceivers()
	require.Equal(t, 1, len(transceivers))
	require.Equal(t, "0", transceivers[0].Mid())

	transceivers = pcAnswer.GetTransceivers()
	require.Equal(t, 1, len(transceivers))
	require.Equal(t, "0", transceivers[0].Mid())

	ctx, cancel = context.WithCancel(context.Background())
	go sendVideoUntilDone(ctx.Done(), t, []*TrackLocalStaticSample{track2})

	remoteTrack2 := <-tracksCh
	cancel()

	require.NoError(t, pcOffer.RemoveTrack(sender2))

	err = signalPair(pcOffer, pcAnswer)
	require.NoError(t, err)
	<-tracksClosed

	assert.Equal(t, "video2", remoteTrack2.ID())
	assert.Equal(t, "pion2", remoteTrack2.StreamID())

	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnection_Renegotiation_RemoveTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
	assert.NoError(t, err)

	sender, err := pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	trackClosed, trackClosedFunc := context.WithCancel(context.Background())

	pcAnswer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		onTrackFiredFunc()

		for {
			if _, _, err := track.ReadRTP(); errors.Is(err, io.EOF) {
				trackClosedFunc()
				return
			}
		}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{vp8Track})

	assert.NoError(t, pcOffer.RemoveTrack(sender))
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-trackClosed.Done()
	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnection_RoleSwitch(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcFirstOfferer, pcSecondOfferer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcFirstOfferer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(pcFirstOfferer, pcSecondOfferer))

	// Add a new Track to the second offerer
	// This asserts that it will match the ordering of the last RemoteDescription, but then also add new Transceivers to the end
	_, err = pcFirstOfferer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
	assert.NoError(t, err)

	_, err = pcSecondOfferer.AddTrack(vp8Track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcSecondOfferer, pcFirstOfferer))
	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{vp8Track})

	closePairNow(t, pcFirstOfferer, pcSecondOfferer)
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
	assert.NoError(t, api.mediaEngine.RegisterDefaultCodecs())

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

	_, err = pcOffer.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

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

		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
		assert.NoError(t, pcOffer.SetLocalDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		assert.NoError(t, pcOffer.SetRemoteDescription(answer))
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	}
	negotiate()
	negotiate()

	pcOffer.ops.Done()
	pcAnswer.ops.Done()
	wg.Wait()

	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnection_Renegotiation_SetLocalDescription(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcOffer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	localTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
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
	assert.Equal(t, 0, len(localTrack.rtpTrack.bindings))

	assert.NoError(t, pcAnswer.SetLocalDescription(answer))

	pcAnswer.ops.Done()
	assert.Equal(t, 1, len(localTrack.rtpTrack.bindings))

	assert.NoError(t, pcOffer.SetRemoteDescription(answer))

	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{localTrack})

	closePairNow(t, pcOffer, pcAnswer)
}

// Issue #346, don't start the SCTP Subsystem if the RemoteDescription doesn't contain one
// Before we would always start it, and re-negotiations would fail because SCTP was in flight
func TestPeerConnection_Renegotiation_NoApplication(t *testing.T) {
	signalPairExcludeDataChannel := func(pcOffer, pcAnswer *PeerConnection) {
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

		assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))
	}

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	pcOfferConnected, pcOfferConnectedCancel := context.WithCancel(context.Background())
	pcOffer.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateConnected {
			pcOfferConnectedCancel()
		}
	})

	pcAnswerConnected, pcAnswerConnectedCancel := context.WithCancel(context.Background())
	pcAnswer.OnICEConnectionStateChange(func(i ICEConnectionState) {
		if i == ICEConnectionStateConnected {
			pcAnswerConnectedCancel()
		}
	})

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionSendrecv})
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionSendrecv})
	assert.NoError(t, err)

	signalPairExcludeDataChannel(pcOffer, pcAnswer)
	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	signalPairExcludeDataChannel(pcOffer, pcAnswer)
	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	<-pcAnswerConnected.Done()
	<-pcOfferConnected.Done()

	assert.Equal(t, pcOffer.SCTP().State(), SCTPTransportStateConnecting)
	assert.Equal(t, pcAnswer.SCTP().State(), SCTPTransportStateConnecting)

	closePairNow(t, pcOffer, pcAnswer)
}

func TestAddDataChannelDuringRenegotiation(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(track)
	assert.NoError(t, err)

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

	assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

	_, err = pcOffer.CreateDataChannel("data-channel", nil)
	assert.NoError(t, err)

	// Assert that DataChannel is in offer now
	offer, err = pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	applicationMediaSectionCount := 0
	for _, d := range offer.parsed.MediaDescriptions {
		if d.MediaName.Media == mediaSectionApplication {
			applicationMediaSectionCount++
		}
	}
	assert.Equal(t, applicationMediaSectionCount, 1)

	onDataChannelFired, onDataChannelFiredFunc := context.WithCancel(context.Background())
	pcAnswer.OnDataChannel(func(*DataChannel) {
		onDataChannelFiredFunc()
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	<-onDataChannelFired.Done()
	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that CreateDataChannel fires OnNegotiationNeeded
func TestNegotiationCreateDataChannel(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	pc.OnNegotiationNeeded(func() {
		defer func() {
			wg.Done()
		}()
	})

	// Create DataChannel, wait until OnNegotiationNeeded is fired
	if _, err = pc.CreateDataChannel("testChannel", nil); err != nil {
		t.Error(err.Error())
	}

	// Wait until OnNegotiationNeeded is fired
	wg.Wait()
	assert.NoError(t, pc.Close())
}

func TestNegotiationNeededRemoveTrack(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	pcOffer.OnNegotiationNeeded(func() {
		wg.Add(1)
		offer, createOfferErr := pcOffer.CreateOffer(nil)
		assert.NoError(t, createOfferErr)

		offerGatheringComplete := GatheringCompletePromise(pcOffer)
		assert.NoError(t, pcOffer.SetLocalDescription(offer))

		<-offerGatheringComplete
		assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))

		answer, createAnswerErr := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, createAnswerErr)

		answerGatheringComplete := GatheringCompletePromise(pcAnswer)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))

		<-answerGatheringComplete
		assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))
		wg.Done()
		wg.Done()
	})

	sender, err := pcOffer.AddTrack(track)
	assert.NoError(t, err)

	assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}))

	wg.Wait()

	wg.Add(1)
	assert.NoError(t, pcOffer.RemoveTrack(sender))

	wg.Wait()

	closePairNow(t, pcOffer, pcAnswer)
}

func TestNegotiationNeededStressOneSided(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcA, pcB, err := newPair()
	assert.NoError(t, err)

	const expectedTrackCount = 500
	ctx, done := context.WithCancel(context.Background())
	pcA.OnNegotiationNeeded(func() {
		count := len(pcA.GetTransceivers())
		assert.NoError(t, signalPair(pcA, pcB))
		if count == expectedTrackCount {
			done()
		}
	})

	for i := 0; i < expectedTrackCount; i++ {
		track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
		assert.NoError(t, err)

		_, err = pcA.AddTrack(track)
		assert.NoError(t, err)
	}
	<-ctx.Done()
	assert.Equal(t, expectedTrackCount, len(pcB.GetTransceivers()))
	closePairNow(t, pcA, pcB)
}

// TestPeerConnection_Renegotiation_DisableTrack asserts that if a remote track is set inactive
// that locally it goes inactive as well
func TestPeerConnection_Renegotiation_DisableTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	// Create two transceivers
	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	transceiver, err := pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	// Assert we have three active transceivers
	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.Equal(t, strings.Count(offer.SDP, "a=sendrecv"), 3)

	// Assert we have two active transceivers, one inactive
	assert.NoError(t, transceiver.Stop())
	offer, err = pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.Equal(t, strings.Count(offer.SDP, "a=sendrecv"), 2)
	assert.Equal(t, strings.Count(offer.SDP, "a=inactive"), 1)

	// Assert that the offer disabled one of our transceivers
	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))
	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.Equal(t, strings.Count(answer.SDP, "a=sendrecv"), 1) // DataChannel
	assert.Equal(t, strings.Count(answer.SDP, "a=recvonly"), 1)
	assert.Equal(t, strings.Count(answer.SDP, "a=inactive"), 1)

	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnection_Renegotiation_Simulcast(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	m := &MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}
	registerSimulcastHeaderExtensions(m, RTPCodecTypeVideo)

	originalRids := []string{"a", "b", "c"}
	signalWithRids := func(sessionDescription string, rids []string) string {
		sessionDescription = strings.SplitAfter(sessionDescription, "a=end-of-candidates\r\n")[0]
		sessionDescription = filterSsrc(sessionDescription)
		for _, rid := range rids {
			sessionDescription += "a=" + sdpAttributeRid + ":" + rid + " send\r\n"
		}
		return sessionDescription + "a=simulcast:send " + strings.Join(rids, ";") + "\r\n"
	}

	var trackMapLock sync.RWMutex
	trackMap := map[string]*TrackRemote{}

	onTrackHandler := func(track *TrackRemote, _ *RTPReceiver) {
		trackMapLock.Lock()
		defer trackMapLock.Unlock()
		trackMap[track.RID()] = track
	}

	sendUntilAllTracksFired := func(vp8Writer *TrackLocalStaticRTP, rids []string) {
		allTracksFired := func() bool {
			trackMapLock.Lock()
			defer trackMapLock.Unlock()

			return len(trackMap) == len(rids)
		}

		for sequenceNumber := uint16(0); !allTracksFired(); sequenceNumber++ {
			time.Sleep(20 * time.Millisecond)

			for ssrc, rid := range rids {
				header := &rtp.Header{
					Version:        2,
					SSRC:           uint32(ssrc),
					SequenceNumber: sequenceNumber,
					PayloadType:    96,
				}
				assert.NoError(t, header.SetExtension(1, []byte("0")))
				assert.NoError(t, header.SetExtension(2, []byte(rid)))

				_, err := vp8Writer.bindings[0].writeStream.WriteRTP(header, []byte{0x00})
				assert.NoError(t, err)
			}
		}
	}

	assertTracksClosed := func(t *testing.T) {
		trackMapLock.Lock()
		defer trackMapLock.Unlock()

		for _, track := range trackMap {
			_, _, err := track.ReadRTP() // Ignore first Read, this is our peeked data
			assert.Nil(t, err)

			_, _, err = track.ReadRTP()
			assert.Equal(t, err, io.EOF)
		}
	}

	t.Run("Disable Transceiver", func(t *testing.T) {
		trackMap = map[string]*TrackRemote{}
		pcOffer, pcAnswer, err := NewAPI(WithMediaEngine(m)).newPair(Configuration{})
		assert.NoError(t, err)

		vp8Writer, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
		assert.NoError(t, err)

		rtpTransceiver, err := pcOffer.AddTransceiverFromTrack(
			vp8Writer,
			RTPTransceiverInit{
				Direction: RTPTransceiverDirectionSendonly,
			},
		)
		assert.NoError(t, err)

		assert.NoError(t, signalPairWithModification(pcOffer, pcAnswer, func(sessionDescription string) string {
			return signalWithRids(sessionDescription, originalRids)
		}))

		pcAnswer.OnTrack(onTrackHandler)
		sendUntilAllTracksFired(vp8Writer, originalRids)

		assert.NoError(t, pcOffer.RemoveTrack(rtpTransceiver.Sender()))
		assert.NoError(t, signalPairWithModification(pcOffer, pcAnswer, func(sessionDescription string) string {
			sessionDescription = strings.SplitAfter(sessionDescription, "a=end-of-candidates\r\n")[0]
			return sessionDescription
		}))

		assertTracksClosed(t)
		closePairNow(t, pcOffer, pcAnswer)
	})

	t.Run("Change RID", func(t *testing.T) {
		trackMap = map[string]*TrackRemote{}
		pcOffer, pcAnswer, err := NewAPI(WithMediaEngine(m)).newPair(Configuration{})
		assert.NoError(t, err)

		vp8Writer, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
		assert.NoError(t, err)

		_, err = pcOffer.AddTransceiverFromTrack(
			vp8Writer,
			RTPTransceiverInit{
				Direction: RTPTransceiverDirectionSendonly,
			},
		)
		assert.NoError(t, err)

		assert.NoError(t, signalPairWithModification(pcOffer, pcAnswer, func(sessionDescription string) string {
			return signalWithRids(sessionDescription, originalRids)
		}))

		pcAnswer.OnTrack(onTrackHandler)
		sendUntilAllTracksFired(vp8Writer, originalRids)

		newRids := []string{"d", "e", "f"}
		assert.NoError(t, signalPairWithModification(pcOffer, pcAnswer, func(sessionDescription string) string {
			scanner := bufio.NewScanner(strings.NewReader(sessionDescription))
			sessionDescription = ""
			for scanner.Scan() {
				l := scanner.Text()
				if strings.HasPrefix(l, "a=rid") || strings.HasPrefix(l, "a=simulcast") {
					continue
				}

				sessionDescription += l + "\n"
			}
			return signalWithRids(sessionDescription, newRids)
		}))

		assertTracksClosed(t)
		closePairNow(t, pcOffer, pcAnswer)
	})
}

func TestPeerConnection_Regegotiation_ReuseTransceiver(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
	assert.NoError(t, err)
	sender, err := pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	assert.Equal(t, len(pcOffer.GetTransceivers()), 1)
	assert.Equal(t, pcOffer.GetTransceivers()[0].getCurrentDirection(), RTPTransceiverDirectionSendonly)
	assert.NoError(t, pcOffer.RemoveTrack(sender))
	assert.Equal(t, pcOffer.GetTransceivers()[0].getCurrentDirection(), RTPTransceiverDirectionSendonly)

	// should not reuse tranceiver
	vp8Track2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
	assert.NoError(t, err)
	sender2, err := pcOffer.AddTrack(vp8Track2)
	assert.NoError(t, err)
	assert.Equal(t, len(pcOffer.GetTransceivers()), 2)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))
	assert.True(t, sender2.rtpTransceiver == pcOffer.GetTransceivers()[1])

	// should reuse first transceiver
	sender, err = pcOffer.AddTrack(vp8Track)
	assert.NoError(t, err)
	assert.Equal(t, len(pcOffer.GetTransceivers()), 2)
	assert.True(t, sender.rtpTransceiver == pcOffer.GetTransceivers()[0])

	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnection_Renegotiation_MidConflict(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerPC, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	answerPC, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	_, err = offerPC.CreateDataChannel("test", nil)
	assert.NoError(t, err)

	_, err = offerPC.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionSendonly})
	assert.NoError(t, err)
	_, err = offerPC.AddTransceiverFromKind(RTPCodecTypeAudio, RtpTransceiverInit{Direction: RTPTransceiverDirectionSendonly})
	assert.NoError(t, err)

	offer, err := offerPC.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, offerPC.SetLocalDescription(offer))
	assert.NoError(t, answerPC.SetRemoteDescription(offer), offer.SDP)
	answer, err := answerPC.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, answerPC.SetLocalDescription(answer))
	assert.NoError(t, offerPC.SetRemoteDescription(answer))
	assert.Equal(t, SignalingStateStable, offerPC.SignalingState())

	tr, err := offerPC.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionSendonly})
	assert.NoError(t, err)
	assert.NoError(t, tr.SetMid("3"))
	_, err = offerPC.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionSendrecv})
	assert.NoError(t, err)
	_, err = offerPC.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, offerPC.Close())
	assert.NoError(t, answerPC.Close())
}

func TestPeerConnection_Regegotiation_AnswerAddsTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	tracksCh := make(chan *TrackRemote)
	pcOffer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		tracksCh <- track
		for {
			if _, _, readErr := track.ReadRTP(); errors.Is(readErr, io.EOF) {
				return
			}
		}
	})

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
	assert.NoError(t, err)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendonly,
	})
	assert.NoError(t, err)

	assert.NoError(t, err)
	_, err = pcAnswer.AddTrack(vp8Track)
	assert.NoError(t, err)
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	ctx, cancel := context.WithCancel(context.Background())

	go sendVideoUntilDone(ctx.Done(), t, []*TrackLocalStaticSample{vp8Track})

	<-tracksCh
	cancel()

	closePairNow(t, pcOffer, pcAnswer)
}

func TestNegotiationNeededWithRecvonlyTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	pcAnswer.OnNegotiationNeeded(wg.Done)

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	if err != nil {
		t.Fatal(err)
	}

	if err := signalPair(pcOffer, pcAnswer); err != nil {
		t.Fatal(err)
	}

	onDataChannel, onDataChannelCancel := context.WithCancel(context.Background())
	pcAnswer.OnDataChannel(func(d *DataChannel) {
		onDataChannelCancel()
	})
	<-onDataChannel.Done()
	wg.Wait()

	closePairNow(t, pcOffer, pcAnswer)
}
