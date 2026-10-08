// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"testing"
	"time"

	"github.com/pion/srtp/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func srtpStarted(pc *PeerConnection) bool {
	_, ok := pc.dtlsTransport.srtpSession.Load().(*srtp.SessionSRTP)

	return ok
}

func waitForMessage(t *testing.T, messages <-chan string, want string) {
	t.Helper()

	select {
	case msg := <-messages:
		assert.Equal(t, want, msg)
	case <-time.After(10 * time.Second):
		require.FailNow(t, "no data channel message", want)
	}
}

func waitForSRTP(t *testing.T, pcs ...*PeerConnection) {
	t.Helper()

	for _, pc := range pcs {
		select {
		case <-pc.dtlsTransport.srtpReady:
		case <-time.After(10 * time.Second):
			require.FailNow(t, "SRTP never started")
		}
	}
}

// A connection with only data channels starts no SRTP sessions and no
// undeclared media processors.
func TestDataChannelOnlyStartsNoMedia(t *testing.T) {
	pcOffer, pcAnswer, err := newPair()
	require.NoError(t, err)
	defer closePairNow(t, pcOffer, pcAnswer)

	messages := make(chan string, 1)
	dc, err := pcOffer.CreateDataChannel("data", nil)
	require.NoError(t, err)
	dc.OnOpen(func() { assert.NoError(t, dc.SendText("hi")) })
	pcAnswer.OnDataChannel(func(d *DataChannel) {
		d.OnMessage(func(msg DataChannelMessage) { messages <- string(msg.Data) })
	})

	require.NoError(t, signalPair(pcOffer, pcAnswer))
	waitForMessage(t, messages, "hi")

	for _, pc := range []*PeerConnection{pcOffer, pcAnswer} {
		assert.False(t, srtpStarted(pc), "SRTP session started")
		assert.False(t, pc.undeclaredStarted.Load(), "undeclared media processors started")
	}
}

// A connection with a track still starts SRTP on both ends.
func TestMediaStartsSRTP(t *testing.T) {
	pcOffer, pcAnswer, err := newPair()
	require.NoError(t, err)
	defer closePairNow(t, pcOffer, pcAnswer)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "audio", "pion")
	require.NoError(t, err)
	_, err = pcOffer.AddTrack(track)
	require.NoError(t, err)

	require.NoError(t, signalPair(pcOffer, pcAnswer))
	waitForSRTP(t, pcOffer, pcAnswer)
	for _, pc := range []*PeerConnection{pcOffer, pcAnswer} {
		assert.True(t, pc.undeclaredStarted.Load())
	}
}

// A connection that starts with only a data channel carries media once a track
// is added and the connection is renegotiated, in either direction.
func TestDataChannelThenMediaRenegotiation(t *testing.T) {
	t.Run("OffererAddsTrack", func(t *testing.T) { testDataChannelThenMedia(t, false) })
	t.Run("AnswererAddsTrack", func(t *testing.T) { testDataChannelThenMedia(t, true) })
}

func testDataChannelThenMedia(t *testing.T, answererSends bool) {
	t.Helper()

	pcOffer, pcAnswer, err := newPair()
	require.NoError(t, err)
	defer closePairNow(t, pcOffer, pcAnswer)

	messages := make(chan string, 4)
	dc, err := pcOffer.CreateDataChannel("data", nil)
	require.NoError(t, err)
	dc.OnOpen(func() { assert.NoError(t, dc.SendText("before")) })
	pcAnswer.OnDataChannel(func(d *DataChannel) {
		d.OnMessage(func(msg DataChannelMessage) { messages <- string(msg.Data) })
	})

	require.NoError(t, signalPair(pcOffer, pcAnswer))
	waitForMessage(t, messages, "before")
	assert.False(t, srtpStarted(pcOffer), "SRTP started before any media")
	assert.False(t, srtpStarted(pcAnswer), "SRTP started before any media")

	sender, receiver := pcOffer, pcAnswer
	if answererSends {
		sender, receiver = pcAnswer, pcOffer
	}
	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	require.NoError(t, err)
	_, err = sender.AddTrack(track)
	require.NoError(t, err)

	gotRTP, gotRTPDone := context.WithTimeout(context.Background(), 20*time.Second)
	defer gotRTPDone()
	receiver.OnTrack(func(remote *TrackRemote, _ *RTPReceiver) {
		if _, _, readErr := remote.ReadRTP(); readErr == nil {
			gotRTPDone()
		}
	})
	require.NoError(t, signalPair(sender, receiver))

	sendVideoUntilDone(t, gotRTP.Done(), []*TrackLocalStaticSample{track})
	require.ErrorIs(t, gotRTP.Err(), context.Canceled, "no RTP packet arrived after renegotiation")
	waitForSRTP(t, pcOffer, pcAnswer)
	for _, pc := range []*PeerConnection{pcOffer, pcAnswer} {
		assert.Eventually(t, pc.undeclaredStarted.Load, 10*time.Second, 10*time.Millisecond,
			"undeclared media processors did not start after renegotiation")
	}

	assert.NoError(t, dc.SendText("after"))
	waitForMessage(t, messages, "after")
}
