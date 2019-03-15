// +build !js

package webrtc

import (
	"math/rand"
	"testing"
)

func TestNewVideoTrack(t *testing.T) {
	m := MediaEngine{}
	m.RegisterCodec(NewRTPVP8Codec(DefaultPayloadTypeVP8, 90000))
	api := NewAPI(WithMediaEngine(m))
	peerConfig := Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peer, _ := api.NewPeerConnection(peerConfig)

	_, err := peer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion")
	if err != nil {
		t.Error("Failed to new video track")
	}
}

func TestNewAudioTrack(t *testing.T) {
	m := MediaEngine{}
	m.RegisterCodec(NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	api := NewAPI(WithMediaEngine(m))
	peerConfig := Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peer, _ := api.NewPeerConnection(peerConfig)

	_, err := peer.NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion")
	if err != nil {
		t.Error("Failed to new audio track")
	}
}

func TestNewTracks(t *testing.T) {
	m := MediaEngine{}
	m.RegisterCodec(NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	m.RegisterCodec(NewRTPVP8Codec(DefaultPayloadTypeVP8, 90000))
	api := NewAPI(WithMediaEngine(m))
	peerConfig := Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peer, _ := api.NewPeerConnection(peerConfig)

	_, err := peer.NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion")
	if err != nil {
		t.Error("Failed to new audio track")
	}

	_, err = peer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion")
	if err != nil {
		t.Error("Failed to new video track")
	}

}

func TestNewTracksWrite(t *testing.T) {
	m := MediaEngine{}
	m.RegisterCodec(NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	m.RegisterCodec(NewRTPVP8Codec(DefaultPayloadTypeVP8, 90000))
	api := NewAPI(WithMediaEngine(m))
	peerConfig := Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peer, _ := api.NewPeerConnection(peerConfig)

	videoTrack, err := peer.NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion")
	if err != nil {
		t.Error("Failed to new video track")
	}

	audioTrack, err := peer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion")
	if err != nil {
		t.Error("Failed to new audio track")
	}
	rtpBuf := make([]byte, 1400)
	_, err = videoTrack.Write(rtpBuf)
	if err != nil {
		t.Error("Failed to write to video track")
	}

	_, err = audioTrack.Write(rtpBuf)
	if err != nil {
		t.Error("Failed to write to audio track")
	}

}
