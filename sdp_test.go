// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v3/test"
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

	t.Run("Session fingerprint wins over media", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "foo bar"}},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "fingerprint", Value: "zoo boo"}}},
			},
		}

		fingerprint, hash, err := extractFingerprint(s)
		assert.NoError(t, err)
		assert.Equal(t, fingerprint, "bar")
		assert.Equal(t, hash, "foo")
	})

	t.Run("Fingerprint from master bundle section", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{
				{Key: "group", Value: "BUNDLE 1 0"},
			},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{
					{Key: "mid", Value: "0"},
					{Key: "fingerprint", Value: "zoo boo"},
				}},
				{Attributes: []sdp.Attribute{
					{Key: "mid", Value: "1"},
					{Key: "fingerprint", Value: "bar foo"},
				}},
			},
		}

		fingerprint, hash, err := extractFingerprint(descr)
		assert.NoError(t, err)
		assert.Equal(t, fingerprint, "foo")
		assert.Equal(t, hash, "bar")
	})

	t.Run("Fingerprint from first media section", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{
					{Key: "mid", Value: "0"},
					{Key: "fingerprint", Value: "zoo boo"},
				}},
				{Attributes: []sdp.Attribute{
					{Key: "mid", Value: "1"},
					{Key: "fingerprint", Value: "bar foo"},
				}},
			},
		}

		fingerprint, hash, err := extractFingerprint(descr)
		assert.NoError(t, err)
		assert.Equal(t, fingerprint, "boo")
		assert.Equal(t, hash, "zoo")
	})
}

func TestExtractICEDetails(t *testing.T) {
	const defaultUfrag = "defaultUfrag"
	const defaultPwd = "defaultPwd"
	const invalidUfrag = "invalidUfrag"
	const invalidPwd = "invalidPwd"

	t.Run("Missing ice-pwd", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: defaultUfrag}}},
			},
		}

		_, err := extractICEDetails(s, nil)
		assert.Equal(t, err, ErrSessionDescriptionMissingIcePwd)
	})

	t.Run("Missing ice-ufrag", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, err := extractICEDetails(s, nil)
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

		details, err := extractICEDetails(s, nil)
		assert.NoError(t, err)
		assert.Equal(t, details.Ufrag, defaultUfrag)
		assert.Equal(t, details.Password, defaultPwd)
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

		details, err := extractICEDetails(s, nil)
		assert.NoError(t, err)
		assert.Equal(t, details.Ufrag, defaultUfrag)
		assert.Equal(t, details.Password, defaultPwd)
	})

	t.Run("ice details at session preferred over media", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{
				{Key: "ice-ufrag", Value: defaultUfrag},
				{Key: "ice-pwd", Value: defaultPwd},
			},
			MediaDescriptions: []*sdp.MediaDescription{
				{
					Attributes: []sdp.Attribute{
						{Key: "ice-ufrag", Value: invalidUfrag},
						{Key: "ice-pwd", Value: invalidPwd},
					},
				},
			},
		}

		details, err := extractICEDetails(descr, nil)
		assert.NoError(t, err)
		assert.Equal(t, details.Ufrag, defaultUfrag)
		assert.Equal(t, details.Password, defaultPwd)
	})

	t.Run("ice details from bundle media section", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{
				{Key: "group", Value: "BUNDLE 5 2"},
			},
			MediaDescriptions: []*sdp.MediaDescription{
				{
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "2"},
						{Key: "ice-ufrag", Value: invalidUfrag},
						{Key: "ice-pwd", Value: invalidPwd},
					},
				},
				{
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "5"},
						{Key: "ice-ufrag", Value: defaultUfrag},
						{Key: "ice-pwd", Value: defaultPwd},
					},
				},
			},
		}

		details, err := extractICEDetails(descr, nil)
		assert.NoError(t, err)
		assert.Equal(t, details.Ufrag, defaultUfrag)
		assert.Equal(t, details.Password, defaultPwd)
	})

	t.Run("ice details from first media section", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					Attributes: []sdp.Attribute{
						{Key: "ice-ufrag", Value: defaultUfrag},
						{Key: "ice-pwd", Value: defaultPwd},
						{Key: "mid", Value: "5"},
					},
				},
				{
					Attributes: []sdp.Attribute{
						{Key: "ice-ufrag", Value: invalidUfrag},
						{Key: "ice-pwd", Value: invalidPwd},
					},
				},
			},
		}

		details, err := extractICEDetails(descr, nil)
		assert.NoError(t, err)
		assert.Equal(t, details.Ufrag, defaultUfrag)
		assert.Equal(t, details.Password, defaultPwd)
	})

	t.Run("Missing pwd at session level", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: "invalidUfrag"}},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: defaultUfrag}, {Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, err := extractICEDetails(s, nil)
		assert.Equal(t, err, ErrSessionDescriptionMissingIcePwd)
	})

	t.Run("Extracts candidate from media section", func(t *testing.T) {
		sdp := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{
				{Key: "group", Value: "BUNDLE video audio"},
			},
			MediaDescriptions: []*sdp.MediaDescription{
				{
					MediaName: sdp.MediaName{
						Media: "audio",
					},
					Attributes: []sdp.Attribute{
						{Key: "ice-ufrag", Value: "ufrag"},
						{Key: "ice-pwd", Value: "pwd"},
						{Key: "ice-options", Value: "google-ice"},
						{Key: "candidate", Value: "1 1 udp 2122162783 192.168.84.254 46492 typ host generation 0"},
					},
				},
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "ice-ufrag", Value: "ufrag"},
						{Key: "ice-pwd", Value: "pwd"},
						{Key: "ice-options", Value: "google-ice"},
						{Key: "mid", Value: "video"},
						{Key: "candidate", Value: "1 1 udp 2122162783 192.168.84.254 46492 typ host generation 0"},
					},
				},
			},
		}

		details, err := extractICEDetails(sdp, nil)
		assert.NoError(t, err)
		assert.Equal(t, details.Ufrag, "ufrag")
		assert.Equal(t, details.Password, "pwd")
		assert.Equal(t, details.Candidates[0].Address, "192.168.84.254")
		assert.Equal(t, details.Candidates[0].Port, uint16(46492))
		assert.Equal(t, details.Candidates[0].Typ, ICECandidateTypeHost)
		assert.Equal(t, details.Candidates[0].SDPMid, "video")
		assert.Equal(t, details.Candidates[0].SDPMLineIndex, uint16(1))
	})
}

func TestSelectCandidateMediaSection(t *testing.T) {
	t.Run("no media section", func(t *testing.T) {
		descr := &sdp.SessionDescription{}

		media, ok := selectCandidateMediaSection(descr)
		assert.False(t, ok)
		assert.Nil(t, media)
	})

	t.Run("no bundle", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "mid", Value: "0"}}},
				{Attributes: []sdp.Attribute{{Key: "mid", Value: "1"}}},
			},
		}

		media, ok := selectCandidateMediaSection(descr)
		assert.True(t, ok)
		assert.NotNil(t, media)
		assert.NotNil(t, media.MediaDescription)
		assert.Equal(t, "0", media.SDPMid)
		assert.Equal(t, uint16(0), media.SDPMLineIndex)
	})

	t.Run("with bundle", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{
				{Key: "group", Value: "BUNDLE 5 2"},
			},
			MediaDescriptions: []*sdp.MediaDescription{
				{
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "2"},
					},
				},
				{
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "5"},
					},
				},
			},
		}

		media, ok := selectCandidateMediaSection(descr)
		assert.True(t, ok)
		assert.NotNil(t, media)
		assert.NotNil(t, media.MediaDescription)
		assert.Equal(t, "5", media.SDPMid)
		assert.Equal(t, uint16(1), media.SDPMLineIndex)
	})
}

func TestTrackDetailsFromSDP(t *testing.T) {
	t.Run("Tracks unknown, audio and video with RTX", func(t *testing.T) {
		descr := &sdp.SessionDescription{
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
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "sendonly"},
						{Key: sdpAttributeRid, Value: "f send pt=97;max-width=1280;max-height=720"},
					},
				},
			},
		}

		tracks := trackDetailsFromSDP(nil, descr)
		assert.Equal(t, 3, len(tracks))
		if trackDetail := trackDetailsForSSRC(tracks, 1000); trackDetail != nil {
			assert.Fail(t, "got the unknown track ssrc:1000 which should have been skipped")
		}
		if track := trackDetailsForSSRC(tracks, 2000); track == nil {
			assert.Fail(t, "missing audio track with ssrc:2000")
		} else {
			assert.Equal(t, RTPCodecTypeAudio, track.kind)
			assert.Equal(t, SSRC(2000), track.ssrcs[0])
			assert.Equal(t, "audio_trk_label", track.streamID)
		}
		if track := trackDetailsForSSRC(tracks, 3000); track == nil {
			assert.Fail(t, "missing video track with ssrc:3000")
		} else {
			assert.Equal(t, RTPCodecTypeVideo, track.kind)
			assert.Equal(t, SSRC(3000), track.ssrcs[0])
			assert.Equal(t, "video_trk_label", track.streamID)
		}
		if track := trackDetailsForSSRC(tracks, 4000); track != nil {
			assert.Fail(t, "got the rtx track ssrc:3000 which should have been skipped")
		}
		if track := trackDetailsForSSRC(tracks, 5000); track == nil {
			assert.Fail(t, "missing video track with ssrc:5000")
		} else {
			assert.Equal(t, RTPCodecTypeVideo, track.kind)
			assert.Equal(t, SSRC(5000), track.ssrcs[0])
			assert.Equal(t, "video_trk_id", track.id)
			assert.Equal(t, "video_stream_id", track.streamID)
		}
	})

	t.Run("inactive and recvonly tracks ignored", func(t *testing.T) {
		descr := &sdp.SessionDescription{
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
		assert.Equal(t, 0, len(trackDetailsFromSDP(nil, descr)))
	})

	t.Run("ssrc-group after ssrc", func(t *testing.T) {
		descr := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "0"},
						{Key: "sendrecv"},
						{Key: "ssrc", Value: "3000 msid:video_trk_label video_trk_guid"},
						{Key: "ssrc", Value: "4000 msid:rtx_trk_label rtx_trck_guid"},
						{Key: "ssrc-group", Value: "FID 3000 4000"},
					},
				},
				{
					MediaName: sdp.MediaName{
						Media: "video",
					},
					Attributes: []sdp.Attribute{
						{Key: "mid", Value: "1"},
						{Key: "sendrecv"},
						{Key: "ssrc-group", Value: "FID 5000 6000"},
						{Key: "ssrc", Value: "5000 msid:video_trk_label video_trk_guid"},
						{Key: "ssrc", Value: "6000 msid:rtx_trk_label rtx_trck_guid"},
					},
				},
			},
		}

		tracks := trackDetailsFromSDP(nil, descr)
		assert.Equal(t, 2, len(tracks))
		assert.Equal(t, SSRC(4000), *tracks[0].repairSsrc)
		assert.Equal(t, SSRC(6000), *tracks[1].repairSsrc)
	})
}

func TestHaveApplicationMediaSection(t *testing.T) {
	t.Run("Audio only", func(t *testing.T) {
		descr := &sdp.SessionDescription{
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

		assert.False(t, haveApplicationMediaSection(descr))
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
	assert.NoError(t, engine.RegisterDefaultCodecs())

	api := NewAPI(WithMediaEngine(engine))

	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	certificate, err := GenerateCertificate(sk)
	assert.NoError(t, err)

	media := []mediaSection{
		{
			id: "video",
			transceivers: []*RTPTransceiver{{
				kind:   RTPCodecTypeVideo,
				api:    api,
				codecs: engine.getCodecsByKind(RTPCodecTypeVideo),
			}},
		},
		{
			id: "audio",
			transceivers: []*RTPTransceiver{{
				kind:   RTPCodecTypeAudio,
				api:    api,
				codecs: engine.getCodecsByKind(RTPCodecTypeAudio),
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
			t.Helper()

			s := &sdp.SessionDescription{}

			dtlsFingerprints, err := certificate.GetFingerprints()
			assert.NoError(t, err)

			s, err = populateSDP(s, false,
				dtlsFingerprints,
				SDPMediaDescriptionFingerprints,
				false, true, engine, sdp.ConnectionRoleActive, []ICECandidate{}, ICEParameters{}, media, ICEGatheringStateNew, nil)
			assert.NoError(t, err)

			sdparray, err := s.Marshal()
			assert.NoError(t, err)

			assert.Equal(t, strings.Count(string(sdparray), "sha-256"), expectedFingerprintCount)
		}
	}

	t.Run("Per-Media Description Fingerprints", fingerprintTest(true, 3))
	t.Run("Per-Session Description Fingerprints", fingerprintTest(false, 1))
}

func TestPopulateSDP(t *testing.T) { //nolint:cyclop,maintidx
	t.Run("rid", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		assert.NoError(t, me.RegisterDefaultCodecs())
		api := NewAPI(WithMediaEngine(me))

		tr := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		tr.setDirection(RTPTransceiverDirectionRecvonly)
		rids := []*simulcastRid{
			{
				id:        "ridkey",
				attrValue: "some",
			},
			{
				id:        "ridPaused",
				attrValue: "some2",
				paused:    true,
			},
		}
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tr}, rids: rids}}

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(
			d,
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			me,
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			mediaSections,
			ICEGatheringStateComplete,
			nil,
		)
		assert.Nil(t, err)

		// Test contains rid map keys
		var ridFound int
		for _, desc := range offerSdp.MediaDescriptions {
			if desc.MediaName.Media != "video" {
				continue
			}
			ridsInSDP := getRids(desc)
			for _, rid := range ridsInSDP {
				if rid.id == "ridkey" && !rid.paused {
					ridFound++
				}
				if rid.id == "ridPaused" && rid.paused {
					ridFound++
				}
			}
		}
		assert.Equal(t, 2, ridFound, "All rid keys should be present")
	})
	t.Run("SetCodecPreferences", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		assert.NoError(t, me.RegisterDefaultCodecs())
		api := NewAPI(WithMediaEngine(me))
		me.pushCodecs(me.videoCodecs, RTPCodecTypeVideo)
		me.pushCodecs(me.audioCodecs, RTPCodecTypeAudio)

		tr := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		tr.setDirection(RTPTransceiverDirectionRecvonly)
		codecErr := tr.SetCodecPreferences([]RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeVP8, 90000, 0, "", nil},
				PayloadType:        96,
			},
		})
		assert.NoError(t, codecErr)

		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tr}}}

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(
			d,
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			me,
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			mediaSections,
			ICEGatheringStateComplete,
			nil,
		)
		assert.Nil(t, err)

		// Test codecs
		foundVP8 := false
		for _, desc := range offerSdp.MediaDescriptions {
			if desc.MediaName.Media != "video" {
				continue
			}
			for _, a := range desc.Attributes {
				if strings.Contains(a.Key, "rtpmap") {
					if a.Value == "98 VP9/90000" {
						t.Fatal("vp9 should not be present in sdp")
					} else if a.Value == "96 VP8/90000" {
						foundVP8 = true
					}
				}
			}
		}
		assert.Equal(t, true, foundVP8, "vp8 should be present in sdp")
	})
	t.Run("ice-lite", func(t *testing.T) {
		se := SettingEngine{}
		se.SetLite(true)

		offerSdp, err := populateSDP(
			&sdp.SessionDescription{},
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			&MediaEngine{},
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			[]mediaSection{},
			ICEGatheringStateComplete,
			nil,
		)
		assert.Nil(t, err)

		var found bool
		// ice-lite is an session-level attribute
		for _, a := range offerSdp.Attributes {
			if a.Key == sdp.AttrKeyICELite {
				// ice-lite does not have value (e.g. ":<value>") and it should be an empty string
				if a.Value == "" {
					found = true

					break
				}
			}
		}
		assert.Equal(t, true, found, "ICELite key should be present")
	})
	t.Run("rejected track", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		registerCodecErr := me.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeType:     MimeTypeVP8,
				ClockRate:    90000,
				Channels:     0,
				SDPFmtpLine:  "",
				RTCPFeedback: nil,
			},
			PayloadType: 96,
		}, RTPCodecTypeVideo)
		assert.NoError(t, registerCodecErr)
		api := NewAPI(WithMediaEngine(me))

		videoTransceiver := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		audioTransceiver := &RTPTransceiver{kind: RTPCodecTypeAudio, api: api, codecs: []RTPCodecParameters{}}
		mediaSections := []mediaSection{
			{id: "video", transceivers: []*RTPTransceiver{videoTransceiver}},
			{id: "audio", transceivers: []*RTPTransceiver{audioTransceiver}},
		}

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(
			d,
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			me,
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			mediaSections,
			ICEGatheringStateComplete,
			nil,
		)
		assert.NoError(t, err)

		// Test codecs
		foundRejectedTrack := false
		for _, desc := range offerSdp.MediaDescriptions {
			if desc.MediaName.Media != "audio" {
				continue
			}
			assert.True(t, desc.ConnectionInformation != nil, "connection information must be provided for rejected tracks")
			assert.Equal(t, desc.MediaName.Formats, []string{"0"}, "rejected tracks have 0 for Formats")
			assert.Equal(t, desc.MediaName.Port, sdp.RangedPort{Value: 0}, "rejected tracks have 0 for Port")
			foundRejectedTrack = true
		}
		assert.Equal(t, true, foundRejectedTrack, "rejected track wasn't present")
	})
	t.Run("allow mixed extmap", func(t *testing.T) {
		se := SettingEngine{}
		offerSdp, err := populateSDP(
			&sdp.SessionDescription{},
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			&MediaEngine{},
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			[]mediaSection{},
			ICEGatheringStateComplete,
			nil,
		)
		assert.Nil(t, err)

		var found bool
		// session-level attribute
		for _, a := range offerSdp.Attributes {
			if a.Key == sdp.AttrKeyExtMapAllowMixed {
				if a.Value == "" {
					found = true

					break
				}
			}
		}
		assert.Equal(t, true, found, "AllowMixedExtMap key should be present")

		offerSdp, err = populateSDP(
			&sdp.SessionDescription{},
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			false, &MediaEngine{},
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			[]mediaSection{},
			ICEGatheringStateComplete,
			nil,
		)
		assert.Nil(t, err)

		found = false
		// session-level attribute
		for _, a := range offerSdp.Attributes {
			if a.Key == sdp.AttrKeyExtMapAllowMixed {
				if a.Value == "" {
					found = true

					break
				}
			}
		}
		assert.Equal(t, false, found, "AllowMixedExtMap key should not be present")
	})
	t.Run("bundle all", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		assert.NoError(t, me.RegisterDefaultCodecs())
		api := NewAPI(WithMediaEngine(me))

		tr := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		tr.setDirection(RTPTransceiverDirectionRecvonly)
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tr}}}

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(
			d,
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			me,
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			mediaSections,
			ICEGatheringStateComplete,
			nil,
		)
		assert.Nil(t, err)

		bundle, ok := offerSdp.Attribute(sdp.AttrKeyGroup)
		assert.True(t, ok)
		assert.Equal(t, "BUNDLE video", bundle)
	})
	t.Run("bundle matched", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		assert.NoError(t, me.RegisterDefaultCodecs())
		api := NewAPI(WithMediaEngine(me))

		tra := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		tra.setDirection(RTPTransceiverDirectionRecvonly)
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tra}}}

		trv := &RTPTransceiver{kind: RTPCodecTypeAudio, api: api, codecs: me.audioCodecs}
		trv.setDirection(RTPTransceiverDirectionRecvonly)
		mediaSections = append(mediaSections, mediaSection{id: "audio", transceivers: []*RTPTransceiver{trv}})

		d := &sdp.SessionDescription{}

		matchedBundle := "audio"
		offerSdp, err := populateSDP(
			d,
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			me,
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			mediaSections,
			ICEGatheringStateComplete,
			&matchedBundle,
		)
		assert.Nil(t, err)

		bundle, ok := offerSdp.Attribute(sdp.AttrKeyGroup)
		assert.True(t, ok)
		assert.Equal(t, "BUNDLE audio", bundle)

		mediaVideo := offerSdp.MediaDescriptions[0]
		mid, ok := mediaVideo.Attribute(sdp.AttrKeyMID)
		assert.True(t, ok)
		assert.Equal(t, "video", mid)
		assert.True(t, mediaVideo.MediaName.Port.Value == 0)
	})
	t.Run("empty bundle group", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		assert.NoError(t, me.RegisterDefaultCodecs())
		api := NewAPI(WithMediaEngine(me))

		tra := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		tra.setDirection(RTPTransceiverDirectionRecvonly)
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tra}}}

		d := &sdp.SessionDescription{}

		matchedBundle := ""
		offerSdp, err := populateSDP(
			d,
			false,
			[]DTLSFingerprint{},
			se.sdpMediaLevelFingerprints,
			se.candidates.ICELite,
			true,
			me,
			connectionRoleFromDtlsRole(defaultDtlsRoleOffer),
			[]ICECandidate{},
			ICEParameters{},
			mediaSections,
			ICEGatheringStateComplete,
			&matchedBundle,
		)
		assert.Nil(t, err)

		_, ok := offerSdp.Attribute(sdp.AttrKeyGroup)
		assert.False(t, ok)
	})
}

func TestGetRIDs(t *testing.T) {
	mediaDescr := []*sdp.MediaDescription{
		{
			MediaName: sdp.MediaName{
				Media: "video",
			},
			Attributes: []sdp.Attribute{
				{Key: "sendonly"},
				{Key: sdpAttributeRid, Value: "f send pt=97;max-width=1280;max-height=720"},
			},
		},
	}

	rids := getRids(mediaDescr[0])

	assert.NotEmpty(t, rids, "Rid mapping should be present")
	found := false
	for _, rid := range rids {
		if rid.id == "f" {
			found = true

			break
		}
	}
	if !found {
		assert.Fail(t, "rid values should contain 'f'")
	}
}

func TestCodecsFromMediaDescription(t *testing.T) {
	t.Run("Codec Only", func(t *testing.T) {
		codecs, err := codecsFromMediaDescription(&sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   "audio",
				Formats: []string{"111"},
			},
			Attributes: []sdp.Attribute{
				{Key: "rtpmap", Value: "111 opus/48000/2"},
			},
		})

		assert.Equal(t, codecs, []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 2, "", []RTCPFeedback{}},
				PayloadType:        111,
			},
		})
		assert.NoError(t, err)
	})

	t.Run("Codec with fmtp/rtcp-fb", func(t *testing.T) {
		codecs, err := codecsFromMediaDescription(&sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   "audio",
				Formats: []string{"111"},
			},
			Attributes: []sdp.Attribute{
				{Key: "rtpmap", Value: "111 opus/48000/2"},
				{Key: "fmtp", Value: "111 minptime=10;useinbandfec=1"},
				{Key: "rtcp-fb", Value: "111 goog-remb"},
				{Key: "rtcp-fb", Value: "111 ccm fir"},
				{Key: "rtcp-fb", Value: "* ccm fir"},
				{Key: "rtcp-fb", Value: "* nack"},
			},
		})

		assert.Equal(t, codecs, []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{
					MimeTypeOpus,
					48000,
					2,
					"minptime=10;useinbandfec=1",
					[]RTCPFeedback{
						{"goog-remb", ""},
						{"ccm", "fir"},
						{"nack", ""},
					},
				},
				PayloadType: 111,
			},
		})
		assert.NoError(t, err)
	})
}

func TestRtpExtensionsFromMediaDescription(t *testing.T) {
	extensions, err := rtpExtensionsFromMediaDescription(&sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media:   "audio",
			Formats: []string{"111"},
		},
		Attributes: []sdp.Attribute{
			{Key: "extmap", Value: "1 " + sdp.ABSSendTimeURI},
			{Key: "extmap", Value: "3 " + sdp.SDESMidURI},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, extensions[sdp.ABSSendTimeURI], 1)
	assert.Equal(t, extensions[sdp.SDESMidURI], 3)
}

// Assert that FEC and RTX SSRCes are present if they are enabled in the MediaEngine.
func Test_SSRC_Groups(t *testing.T) {
	const offerWithRTX = `v=0
o=- 930222930247584370 1727933945 IN IP4 0.0.0.0
s=-
t=0 0
a=msid-semantic:WMS*
a=fingerprint:sha-256 11:3F:1C:8D:D4:1D:8D:E7:E1:3E:AF:38:06:0D:1D:40:22:DC:FE:C9:93:E4:80:D8:0B:17:9F:2E:C1:CA:C8:3D
a=extmap-allow-mixed
a=group:BUNDLE 0 1
m=audio 9 UDP/TLS/RTP/SAVPF 101
c=IN IP4 0.0.0.0
a=setup:actpass
a=mid:0
a=ice-ufrag:yIgpPUMarFReduuM
a=ice-pwd:VmnVaqCByWiOTatFoDBbMGhSFGlsxviz
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:101 opus/90000
a=rtcp-fb:101 transport-cc
a=extmap:4 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=ssrc:3566446228 cname:stream-id
a=ssrc:3566446228 msid:stream-id audio-id
a=ssrc:3566446228 mslabel:stream-id
a=ssrc:3566446228 label:audio-id
a=msid:stream-id audio-id
a=sendrecv
m=video 9 UDP/TLS/RTP/SAVPF 96 97
c=IN IP4 0.0.0.0
a=setup:actpass
a=mid:1
a=ice-ufrag:yIgpPUMarFReduuM
a=ice-pwd:VmnVaqCByWiOTatFoDBbMGhSFGlsxviz
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtcp-fb:96 transport-cc
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
a=ssrc-group:FID 1701050765 2578535262
a=ssrc:1701050765 cname:stream-id
a=ssrc:1701050765 msid:stream-id track-id
a=ssrc:1701050765 mslabel:stream-id
a=ssrc:1701050765 label:track-id
a=msid:stream-id track-id
a=sendrecv
`

	const offerNoRTX = `v=0
o=- 930222930247584370 1727933945 IN IP4 0.0.0.0
s=-
t=0 0
a=msid-semantic:WMS*
a=fingerprint:sha-256 11:3F:1C:8D:D4:1D:8D:E7:E1:3E:AF:38:06:0D:1D:40:22:DC:FE:C9:93:E4:80:D8:0B:17:9F:2E:C1:CA:C8:3D
a=extmap-allow-mixed
a=group:BUNDLE 0 1
m=audio 9 UDP/TLS/RTP/SAVPF 101
a=mid:0
a=ice-ufrag:yIgpPUMarFReduuM
a=ice-pwd:VmnVaqCByWiOTatFoDBbMGhSFGlsxviz
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:101 opus/90000
a=rtcp-fb:101 transport-cc
a=extmap:4 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=ssrc:3566446228 cname:stream-id
a=ssrc:3566446228 msid:stream-id audio-id
a=ssrc:3566446228 mslabel:stream-id
a=ssrc:3566446228 label:audio-id
a=msid:stream-id audio-id
a=sendrecv
m=video 9 UDP/TLS/RTP/SAVPF 96
c=IN IP4 0.0.0.0
a=setup:actpass
a=mid:1
a=ice-ufrag:yIgpPUMarFReduuM
a=ice-pwd:VmnVaqCByWiOTatFoDBbMGhSFGlsxviz
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtcp-fb:96 transport-cc
a=ssrc-group:FID 1701050765 2578535262
a=ssrc:1701050765 cname:stream-id
a=ssrc:1701050765 msid:stream-id track-id
a=ssrc:1701050765 mslabel:stream-id
a=ssrc:1701050765 label:track-id
a=msid:stream-id track-id
a=sendrecv
`
	defer test.CheckRoutines(t)()

	for _, testCase := range []struct {
		name                   string
		enableRTXInMediaEngine bool
		rtxExpected            bool
		remoteOffer            string
	}{
		{"Offer", true, true, ""},
		{"Offer no Local Groups", false, false, ""},
		{"Answer", true, true, offerWithRTX},
		{"Answer No Local Groups", false, false, offerWithRTX},
		{"Answer No Remote Groups", true, false, offerNoRTX},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			checkRTXSupport := func(s *sdp.SessionDescription) {
				// RTX is never enabled for audio
				assert.Nil(t, trackDetailsFromSDP(nil, s)[0].repairSsrc)

				// RTX is conditionally enabled for video
				if testCase.rtxExpected {
					assert.NotNil(t, trackDetailsFromSDP(nil, s)[1].repairSsrc)
				} else {
					assert.Nil(t, trackDetailsFromSDP(nil, s)[1].repairSsrc)
				}
			}

			me := &MediaEngine{}
			assert.NoError(t, me.RegisterCodec(RTPCodecParameters{
				RTPCodecCapability: RTPCodecCapability{
					MimeType:     MimeTypeOpus,
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  "",
					RTCPFeedback: nil,
				},
				PayloadType: 101,
			}, RTPCodecTypeAudio))
			assert.NoError(t, me.RegisterCodec(RTPCodecParameters{
				RTPCodecCapability: RTPCodecCapability{
					MimeType:     MimeTypeVP8,
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  "",
					RTCPFeedback: nil,
				},
				PayloadType: 96,
			}, RTPCodecTypeVideo))
			if testCase.enableRTXInMediaEngine {
				assert.NoError(t, me.RegisterCodec(RTPCodecParameters{
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     MimeTypeRTX,
						ClockRate:    90000,
						Channels:     0,
						SDPFmtpLine:  "apt=96",
						RTCPFeedback: nil,
					},
					PayloadType: 97,
				}, RTPCodecTypeVideo))
			}

			peerConnection, err := NewAPI(WithMediaEngine(me)).NewPeerConnection(Configuration{})
			assert.NoError(t, err)

			audioTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "audio-id", "stream-id")
			assert.NoError(t, err)

			_, err = peerConnection.AddTrack(audioTrack)
			assert.NoError(t, err)

			videoTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video-id", "stream-id")
			assert.NoError(t, err)

			_, err = peerConnection.AddTrack(videoTrack)
			assert.NoError(t, err)

			if testCase.remoteOffer == "" {
				offer, err := peerConnection.CreateOffer(nil)
				assert.NoError(t, err)
				checkRTXSupport(offer.parsed)
			} else {
				assert.NoError(t, peerConnection.SetRemoteDescription(SessionDescription{
					Type: SDPTypeOffer, SDP: testCase.remoteOffer,
				}))
				answer, err := peerConnection.CreateAnswer(nil)
				assert.NoError(t, err)
				checkRTXSupport(answer.parsed)
			}

			assert.NoError(t, peerConnection.Close())
		})
	}
}
