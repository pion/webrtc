// +build !js

package webrtc

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStream(t *testing.T) {
	s := Stream{}
	track, err := NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion", NewRTPVP8Codec(DefaultPayloadTypeVP8, 90000))
	assert.NoError(t, err)

	onAddTrackCalled := make(chan interface{})
	s.OnAddTrack(func(t *Track) {
		close(onAddTrackCalled)
	})
	s.addTrack(track)
	<-onAddTrackCalled

	assert.Equal(t, track, s.GetTrackByID(track.ID()))
	assert.Nil(t, s.GetTrackByID("bad"))
	assert.Len(t, s.GetAudioTracks(), 0)
	assert.Len(t, s.GetVideoTracks(), 1)
	assert.Equal(t, track, s.GetVideoTracks()[0])
	assert.Len(t, s.GetTracks(), 1)
	assert.Equal(t, track, s.GetTracks()[0])

	onRemoveTrackCalled := make(chan interface{})
	s.OnRemoveTrack(func(t *Track) {
		close(onRemoveTrackCalled)
	})
	s.removeTrack(track)
	<-onRemoveTrackCalled

	// Shouldn't call OnRemoveTrack again
	s.removeTrack(track)
}

func TestStreamWithMultipleTracks(t *testing.T) {
	s := Stream{}
	video, err := NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion", NewRTPVP8Codec(DefaultPayloadTypeVP8, 90000))
	assert.NoError(t, err)

	s.addTrack(video)

	assert.Len(t, s.GetAudioTracks(), 0)
	assert.Len(t, s.GetVideoTracks(), 1)
	assert.Len(t, s.GetTracks(), 1)

	audio, err := NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion", NewRTPOpusCodec(DefaultPayloadTypeOpus, 4800))
	assert.NoError(t, err)

	s.addTrack(audio)

	assert.Len(t, s.GetAudioTracks(), 1)
	assert.Len(t, s.GetVideoTracks(), 1)
	assert.Len(t, s.GetTracks(), 2)
}
