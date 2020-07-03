// +build !js

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/url"
	"strings"
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

func TestMediaDescriptionFingerprints(t *testing.T) {
	engine := &MediaEngine{}
	engine.RegisterCodec(NewRTPH264Codec(DefaultPayloadTypeH264, 90000))
	engine.RegisterCodec(NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))

	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	certificate, err := GenerateCertificate(sk)
	assert.NoError(t, err)

	media := []mediaSection{
		{
			id: "video",
			transceivers: []*RTPTransceiver{{
				kind: RTPCodecTypeVideo,
			}},
		},
		{
			id: "audio",
			transceivers: []*RTPTransceiver{{
				kind: RTPCodecTypeAudio,
			}},
		},
		{
			id:   "application",
			data: true,
		},
	}

	for i := 0; i < 2; i++ {
		media[i].transceivers[0].setSender(&RTPSender{})
		media[i].transceivers[0].setDirection(RTPTransceiverDirectionSendonly)
	}

	fingerprintTest := func(SDPMediaDescriptionFingerprints bool, expectedFingerprintCount int) func(t *testing.T) {
		return func(t *testing.T) {
			s := &sdp.SessionDescription{}

			dtlsFingerprints, err := certificate.GetFingerprints()
			assert.NoError(t, err)

			s, err = populateSDP(s, false,
				dtlsFingerprints,
				SDPMediaDescriptionFingerprints,
				false, engine, sdp.ConnectionRoleActive, []ICECandidate{}, ICEParameters{}, media, ICEGatheringStateNew, nil)
			assert.NoError(t, err)

			sdparray, err := s.Marshal()
			assert.NoError(t, err)

			assert.Equal(t, strings.Count(string(sdparray), "sha-256"), expectedFingerprintCount)
		}
	}

	t.Run("Per-Media Description Fingerprints", fingerprintTest(true, 3))
	t.Run("Per-Session Description Fingerprints", fingerprintTest(false, 1))
}

func TestPopulateSDP(t *testing.T) {
	t.Run("Offer", func(t *testing.T) {
		transportCCURL, _ := url.Parse(sdp.TransportCCURI)
		absSendURL, _ := url.Parse(sdp.ABSSendTimeURI)

		globalExts := []sdp.ExtMap{
			{
				URI: transportCCURL,
			},
		}
		videoExts := []sdp.ExtMap{
			{
				URI: absSendURL,
			},
		}
		tr := &RTPTransceiver{kind: RTPCodecTypeVideo}
		tr.setDirection(RTPTransceiverDirectionSendrecv)
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tr}}}

		se := SettingEngine{}
		se.AddSDPExtensions(SDPSectionGlobal, globalExts)
		se.AddSDPExtensions(SDPSectionVideo, videoExts)

		m := MediaEngine{}
		m.RegisterDefaultCodecs()

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, &m, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, se.getSDPExtensions())
		assert.Nil(t, err)

		// Check global extensions
		var found bool
		for _, a := range offerSdp.Attributes {
			if strings.Contains(a.Key, transportCCURL.String()) {
				found = true
				break
			}
		}
		assert.Equal(t, true, found, "Global extension should be present")

		// Check video extension
		found = false
		var foundGlobal bool
		for _, desc := range offerSdp.MediaDescriptions {
			if desc.MediaName.Media != mediaNameVideo {
				continue
			}
			for _, a := range desc.Attributes {
				if strings.Contains(a.Key, absSendURL.String()) {
					found = true
				}
				if strings.Contains(a.Key, transportCCURL.String()) {
					foundGlobal = true
				}
			}
		}
		assert.Equal(t, true, found, "Video extension should be present")
		// Test video does not contain global
		assert.Equal(t, false, foundGlobal, "Global extension should not be present in video section")
	})
}

func TestMatchedAnswerExt(t *testing.T) {
	s := &sdp.SessionDescription{
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media: "video",
				},
				Attributes: []sdp.Attribute{
					{Key: "sendrecv"},
					{Key: "ssrc", Value: "2000"},
					{Key: "extmap", Value: "5 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"},
				},
			},
		},
	}

	transportCCURL, _ := url.Parse(sdp.TransportCCURI)
	absSendURL, _ := url.Parse(sdp.ABSSendTimeURI)

	exts := []sdp.ExtMap{
		{
			URI: transportCCURL,
		},
		{
			URI: absSendURL,
		},
	}

	se := SettingEngine{}
	se.AddSDPExtensions(SDPSectionVideo, exts)

	ansMaps, err := matchedAnswerExt(s, se.getSDPExtensions())
	if err != nil {
		t.Fatalf("Ext parse error %v", err)
	}

	if maps := ansMaps[SDPSectionVideo]; maps != nil {
		// Check answer contains intersect of remote and local
		// Abs send time should be only extenstion
		assert.Equal(t, 1, len(maps), "Only one extension should be active")
		assert.Equal(t, absSendURL, maps[0].URI, "Only abs-send-time should be active")

		// Check answer uses remote IDs
		assert.Equal(t, 5, maps[0].Value, "Should use remote ext ID")
	} else {
		t.Fatal("No video ext maps found")
	}
}
