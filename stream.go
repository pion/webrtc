// +build !js

package webrtc

import "sync"

// Stream represents a collection of Tracks
type Stream struct {
	mu                   sync.RWMutex
	ID                   string
	tracks               []*Track
	onAddTrackHandler    func(*Track)
	onRemoveTrackHandler func(*Track)
}

// addTrack adds the given Track to this Stream.
func (s *Stream) addTrack(track *Track) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tracks = append(s.tracks, track)

	if s.onAddTrackHandler != nil {
		s.onAddTrackHandler(track)
	}
}

// OnAddTrack is called when a track is added to this stream.
func (s *Stream) OnAddTrack(f func(*Track)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAddTrackHandler = f
}

// removeTrack removes the given Track to this Stream.
func (s *Stream) removeTrack(track *Track) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.tracks {
		if track.ID() == t.ID() {
			copy(s.tracks[i:], s.tracks[i+1:])
			s.tracks[len(s.tracks)-1] = nil
			s.tracks = s.tracks[:len(s.tracks)-1]

			if s.onRemoveTrackHandler != nil {
				s.onRemoveTrackHandler(track)
			}

			// track removed, end loop
			break
		}
	}
}

// OnRemoveTrack is called when a track is removed from this stream.
func (s *Stream) OnRemoveTrack(f func(*Track)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRemoveTrackHandler = f
}

// GetTrackByID get the Track with given id from this Stream if it exists.
func (s *Stream) GetTrackByID(id string) *Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, track := range s.tracks {
		if track.ID() == id {
			return track
		}
	}
	return nil
}

// GetAudioTracks returns a sequence of Track objects representing the audio tracks in this stream.
func (s *Stream) GetAudioTracks() []*Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var audioTracks []*Track
	for _, track := range s.tracks {
		if track.Kind() == RTPCodecTypeAudio {
			audioTracks = append(audioTracks, track)
		}
	}
	return audioTracks
}

// GetVideoTracks returns a sequence of Track objects representing the video tracks in this stream.
func (s *Stream) GetVideoTracks() []*Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var videoTracks []*Track
	for _, track := range s.tracks {
		if track.Kind() == RTPCodecTypeVideo {
			videoTracks = append(videoTracks, track)
		}
	}
	return videoTracks
}

// GetTracks returns a sequence of Track objects representing the tracks in this stream.
func (s *Stream) GetTracks() []*Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tracks
}
