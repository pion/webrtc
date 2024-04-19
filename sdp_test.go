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

		_, _, _, err := extractICEDetails(s, nil)
		assert.Equal(t, err, ErrSessionDescriptionMissingIcePwd)
	})

	t.Run("Missing ice-ufrag", func(t *testing.T) {
		s := &sdp.SessionDescription{
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, _, _, err := extractICEDetails(s, nil)
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

		ufrag, pwd, _, err := extractICEDetails(s, nil)
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

		ufrag, pwd, _, err := extractICEDetails(s, nil)
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

		_, _, _, err := extractICEDetails(s, nil)
		assert.Equal(t, err, ErrSessionDescriptionConflictingIceUfrag)
	})

	t.Run("Conflict pwd", func(t *testing.T) {
		s := &sdp.SessionDescription{
			Attributes: []sdp.Attribute{{Key: "ice-pwd", Value: "invalidPwd"}},
			MediaDescriptions: []*sdp.MediaDescription{
				{Attributes: []sdp.Attribute{{Key: "ice-ufrag", Value: defaultUfrag}, {Key: "ice-pwd", Value: defaultPwd}}},
			},
		}

		_, _, _, err := extractICEDetails(s, nil)
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

		tracks := trackDetailsFromSDP(nil, s)
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

func TestPopulateSDP(t *testing.T) {
	t.Run("rid", func(t *testing.T) {
		se := SettingEngine{}

		me := &MediaEngine{}
		assert.NoError(t, me.RegisterDefaultCodecs())
		api := NewAPI(WithMediaEngine(me))

		tr := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		tr.setDirection(RTPTransceiverDirectionRecvonly)
		ridMap := map[string]*simulcastRid{
			"ridkey": {
				attrValue: "some",
			},
			"ridPaused": {
				attrValue: "some2",
				paused:    true,
			},
		}
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{tr}, ridMap: ridMap}}

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, me, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, nil)
		assert.Nil(t, err)

		// Test contains rid map keys
		var ridFound int
		for _, desc := range offerSdp.MediaDescriptions {
			if desc.MediaName.Media != "video" {
				continue
			}
			ridInSDP := getRids(desc)
			if ridKey, ok := ridInSDP["ridkey"]; ok && !ridKey.paused {
				ridFound++
			}
			if ridPaused, ok := ridInSDP["ridPaused"]; ok && ridPaused.paused {
				ridFound++
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

		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, me, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, nil)
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

		offerSdp, err := populateSDP(&sdp.SessionDescription{}, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, &MediaEngine{}, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, []mediaSection{}, ICEGatheringStateComplete, nil)
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
			RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo)
		assert.NoError(t, registerCodecErr)
		api := NewAPI(WithMediaEngine(me))

		videoTransceiver := &RTPTransceiver{kind: RTPCodecTypeVideo, api: api, codecs: me.videoCodecs}
		audioTransceiver := &RTPTransceiver{kind: RTPCodecTypeAudio, api: api, codecs: []RTPCodecParameters{}}
		mediaSections := []mediaSection{{id: "video", transceivers: []*RTPTransceiver{videoTransceiver}}, {id: "audio", transceivers: []*RTPTransceiver{audioTransceiver}}}

		d := &sdp.SessionDescription{}

		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, me, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, nil)
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
		offerSdp, err := populateSDP(&sdp.SessionDescription{}, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, &MediaEngine{}, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, []mediaSection{}, ICEGatheringStateComplete, nil)
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

		offerSdp, err = populateSDP(&sdp.SessionDescription{}, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, false, &MediaEngine{}, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, []mediaSection{}, ICEGatheringStateComplete, nil)
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

		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, me, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, nil)
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
		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, me, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, &matchedBundle)
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
		offerSdp, err := populateSDP(d, false, []DTLSFingerprint{}, se.sdpMediaLevelFingerprints, se.candidates.ICELite, true, me, connectionRoleFromDtlsRole(defaultDtlsRoleOffer), []ICECandidate{}, ICEParameters{}, mediaSections, ICEGatheringStateComplete, &matchedBundle)
		assert.Nil(t, err)

		_, ok := offerSdp.Attribute(sdp.AttrKeyGroup)
		assert.False(t, ok)
	})
}

func TestGetRIDs(t *testing.T) {
	m := []*sdp.MediaDescription{
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

	rids := getRids(m[0])

	assert.NotEmpty(t, rids, "Rid mapping should be present")
	if _, ok := rids["f"]; !ok {
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
				RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", []RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}}},
				PayloadType:        111,
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
