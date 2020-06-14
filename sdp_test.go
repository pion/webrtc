// +build !js

package webrtc

import (
	"testing"

	"github.com/pion/sdp/v2"
	"github.com/stretchr/testify/assert"
)

func TestExtractFingerprint(t *testing.T) {
	t.Run("Good Session Fingerprint", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "foo bar"}},
		}

		fingerprint, hash, err := extractFingerprint(s)
		assert.NoError(t, err)
		assert.Equal(t, fingerprint, "bar")
		assert.Equal(t, hash, "foo")
	})

	t.Run("Good Media Fingerprint", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "foo bar"}}},
			},
		}

		fingerprint, hash, err := extractFingerprint(s)
		assert.NoError(t, err)
		assert.Equal(t, fingerprint, "bar")
		assert.Equal(t, hash, "foo")
	})

	t.Run("No Fingerprint", func(t *testing.T) {
		s := &sdp.SessionDescription{}

		_, _, err := extractFingerprint(s)
		assert.Equal(t, ErrSessionDescriptionNoFingerprint, err)
	})

	t.Run("Invalid Fingerprint", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "foo"}},
		}

		_, _, err := extractFingerprint(s)
		assert.Equal(t, ErrSessionDescriptionInvalidFingerprint, err)
	})

	t.Run("Conflicting Fingerprint", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "foo"}},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "foo blah"}}},
			},
		}

		_, _, err := extractFingerprint(s)
		assert.Equal(t, ErrSessionDescriptionConflictingFingerprints, err)
	})
}

func TestExtractICEDetails(t *testing.T) {
	const defaultUfrag = "defaultPwd"
	const defaultPwd = "defaultUfrag"

	t.Run("Missing ice-pwd", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: defaultUfrag}}},
			},
		}

		_, _, _, err := extractICEDetails(s)
		assert.Equal(t, err, ErrSessionDescriptionMissingIcePwd)
	})

	t.Run("Missing ice-ufrag", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, _, _, err := extractICEDetails(s)
		assert.Equal(t, err, ErrSessionDescriptionMissingIceUfrag)
	})

	t.Run("ice details at session level", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{
				{Key: "ice-ufrag", Value: defaultUfrag},
				{Key: "ice-pwd", Value: defaultPwd},
			},
			MediaDescriptions: []*sdp.MediaDescription{},
		}

		ufrag, pwd, _, err := extractICEDetails(s)
		assert.Equal(t, ufrag, defaultUfrag)
		assert.Equal(t, pwd, defaultPwd)
		assert.NoError(t, err)
	})

	t.Run("ice details at media level", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					Attributes: []sdp.Attribute{
						{Key: "ice-ufrag", Value: defaultUfrag},
						{Key: "ice-pwd", Value: defaultPwd},
					},
				},
			},
		}

		ufrag, pwd, _, err := extractICEDetails(s)
		assert.Equal(t, ufrag, defaultUfrag)
		assert.Equal(t, pwd, defaultPwd)
		assert.NoError(t, err)
	})

	t.Run("Conflict ufrag", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: "invalidUfrag"}},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: defaultUfrag}, {Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, _, _, err := extractICEDetails(s)
		assert.Equal(t, err, ErrSessionDescriptionConflictingIceUfrag)
	})

	t.Run("Conflict pwd", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "ice-pwd", Value: "invalidPwd"}},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: defaultUfrag}, {Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, _, _, err := extractICEDetails(s)
		assert.Equal(t, err, ErrSessionDescriptionConflictingIcePwd)
	})
}

func TestTrackDetailsFromSDP(t *testing.T) {
	t.Run("Tracks unknown, audio and video with RTX", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					MediaName: sdp.MediaName{
						Media: "foobar",
					},
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "0"},
						{Key: "sendrecv"},
						{Key: "ssrc", Value: "1000 msid:unknown_trk_label unknown_trk_guid"},
					},
				},
				{
					MediaName: sdp.MediaName{
						Media: "audio",
					},
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "1"},
						{Key: "sendrecv"},
						{Key: "ssrc", Value: "2000 msid:audio_trk_label audio_trk_guid"},
					},
				},
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "2"},
						{Key: "sendrecv"},
						{Key: "ssrc-group", Value: "FID 3000 4000"},
						{Key: "ssrc", Value: "3000 msid:video_trk_label video_trk_guid"},
						{Key: "ssrc", Value: "4000 msid:rtx_trk_label rtx_trck_guid"},
					},
				},
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "3"},
						{Key: "sendonly"},
						{Key: "msid", Value: "video_stream_id video_trk_id"},
						{Key: "ssrc", Value: "5000"},
					},
				},
			},
		}

		tracks := trackDetailsFromSDP(nil, s)
		assert.Equal(t, 3, len(tracks))
		if _, ok := tracks[1000]; ok {
			assert.Fail(t, "got the unknown track ssrc:1000 which should have been skipped")
		}
		if track, ok := tracks[2000]; !ok {
			assert.Fail(t, "missing audio track with ssrc:2000")
		} else {
			assert.Equal(t, RTPCodecTypeAudio, track.kind)
			assert.Equal(t, uint32(2000), track.ssrc)
			assert.Equal(t, "audio_trk_label", track.label)
		}
		if track, ok := tracks[3000]; !ok {
			assert.Fail(t, "missing video track with ssrc:3000")
		} else {
			assert.Equal(t, RTPCodecTypeVideo, track.kind)
			assert.Equal(t, uint32(3000), track.ssrc)
			assert.Equal(t, "video_trk_label", track.label)
		}
		if _, ok := tracks[4000]; ok {
			assert.Fail(t, "got the rtx track ssrc:3000 which should have been skipped")
		}
		if track, ok := tracks[5000]; !ok {
			assert.Fail(t, "missing video track with ssrc:5000")
		} else {
			assert.Equal(t, RTPCodecTypeVideo, track.kind)
			assert.Equal(t, uint32(5000), track.ssrc)
			assert.Equal(t, "video_trk_id", track.id)
			assert.Equal(t, "video_stream_id", track.label)
		}
	})

	t.Run("inactive and recvonly tracks ignored", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "inactive"},
						{Key: "ssrc", Value: "6000"},
					},
				},
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "recvonly"},
						{Key: "ssrc", Value: "7000"},
					},
				},
			},
		}

		assert.Equal(t, 0, len(trackDetailsFromSDP(nil, s)))
	})
}

func TestHaveApplicationMediaSection(t *testing.T) {
	t.Run("Audio only", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					MediaName: sdp.MediaName{
						Media: "audio",
					},
					Attributes: []sdp.Attribute{
						{Key: "sendrecv"},
						{Key: "ssrc", Value: "2000"},
					},
				},
			},
		}

		assert.False(t, haveApplicationMediaSection(s))
	})

	t.Run("Application", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					MediaName: sdp.MediaName{
						Media: mediaSectionApplication,
					},
				},
			},
		}

		assert.True(t, haveApplicationMediaSection(s))
	})
}
