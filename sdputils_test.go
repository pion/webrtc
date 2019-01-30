package webrtc

import (
	"testing"

	"github.com/pions/sdp/v2"
)

func TestParseRtpDecodingParameters_Success(t *testing.T) {
	tt := []struct {
		in       *sdp.MediaDescription
		expected []RTPDecodingParameters
	}{
		{
			in: &sdp.MediaDescription{
				Attributes: []sdp.Attribute{
					{Key: "ssrc", Value: "2520107483 cname:{c830ce58-b0d6-4f95-bb9f-8722bb1eba28}"},
					{Key: "ssrc", Value: "2520107483 bla:{c830ce58-b0d6-4f95-bb9f-8722bb1eba28}"},
				},
			},
			expected: []RTPDecodingParameters{
				{RTPCodingParameters{SSRC: 2520107483}},
			},
		},
	}

	for _, tc := range tt {
		res, err := sdpParseRTPDecodingParameters(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if len(res) != len(tc.expected) {
			t.Errorf("wrong number of parameters: got %d expected %d", len(res), len(tc.expected))
		}
		for i, a := range res {
			e := tc.expected[i]
			if a.SSRC != e.SSRC {
				t.Errorf("wrong ssrc: got %d expected %d", a.SSRC, e.SSRC)
			}
		}
	}
}

func TestParseRtpParameters_Success(t *testing.T) {
	tt := []struct {
		in       *sdp.MediaDescription
		expected RTPParameters
	}{
		{
			in: &sdp.MediaDescription{
				MediaName: sdp.MediaName{
					Formats: []string{"120", "109"},
				},
				Attributes: []sdp.Attribute{
					{Key: "fmtp", Value: "120 max-fs=12288;max-fr=60"},
					{Key: "rtcp-fb", Value: "120 nack"},
					{Key: "rtcp-fb", Value: "tcp-fb:120 nack pli"},
					{Key: "rtpmap", Value: "120 VP8/90000"},
					{Key: "fmtp", Value: "109 maxplaybackrate=48000;stereo=1;useinbandfec=1"},
					{Key: "rtpmap", Value: "109 opus/48000/2"},
					{Key: "extmap", Value: "1 urn:ietf:params:rtp-hdrext:ssrc-audio-level"},
					{Key: "extmap", Value: "2/recvonly urn:ietf:params:rtp-hdrext:csrc-audio-level"},
				},
			},
			expected: RTPParameters{
				Codecs: []RTPCodecParameters{
					{
						Name:        "VP8",
						PayloadType: 120,
						ClockRate:   90000,
						Channels:    0,
						RTCPFeedback: []RTCPFeedback{
							{
								Type: "nack",
							},
							{
								Type:      "nack",
								Parameter: "pli",
							},
						},
						Parameters: map[string]string{
							"max-fs": "12288",
							"max-fr": "60",
						},
					},
					{
						Name:        "opus",
						PayloadType: 109,
						ClockRate:   48000,
						Channels:    2,
						Parameters: map[string]string{
							"maxplaybackrate": "48000",
							"stereo":          "1",
							"useinbandfec":    "1",
						},
					},
				},
				HeaderExtensions: []RTPHeaderExtensionParameters{
					{
						ID:        1,
						direction: "sendrecv",
						URI:       "urn:ietf:params:rtp-hdrext:ssrc-audio-level",
					}, {
						ID:        2,
						direction: "recvonly",
						URI:       "urn:ietf:params:rtp-hdrext:csrc-audio-level",
					},
				},
			},
		},
	}

	for _, tc := range tt {
		res, err := sdpParseRTPParameters(tc.in)
		if err != nil {
			t.Fatal(err)
		}

		if len(res.Codecs) != len(tc.expected.Codecs) {
			t.Errorf("wrong number of codecs: got %d expected %d", len(res.Codecs), len(tc.expected.Codecs))
		}

		for i, a := range res.Codecs {
			e := tc.expected.Codecs[i]
			if a.Name != e.Name {
				t.Errorf("wrong name: got %s expected %s", a.Name, e.Name)
			}
			if a.PayloadType != e.PayloadType {
				t.Errorf("wrong payload type: got %d expected %d", a.PayloadType, e.PayloadType)
			}
			if a.ClockRate != e.ClockRate {
				t.Errorf("wrong clock rate: got %d expected %d", a.ClockRate, e.ClockRate)
			}
			if a.Channels != e.Channels {
				t.Errorf("wrong channels: got %d expected %d", a.Channels, e.Channels)
			}

			if len(a.RTCPFeedback) != len(e.RTCPFeedback) {
				t.Errorf("wrong number of feedbacks: got %d expected %d", len(a.RTCPFeedback), len(e.RTCPFeedback))
			}

			for j, fba := range a.RTCPFeedback {
				fbe := e.RTCPFeedback[j]
				if fba.Type != fbe.Type {
					t.Errorf("wrong type: got %s expected %s", fba.Type, fbe.Type)
				}
				if fba.Parameter != fbe.Parameter {
					t.Errorf("wrong parameter: got %s expected %s", fba.Parameter, fbe.Parameter)
				}
			}

			if len(a.Parameters) != len(e.Parameters) {
				t.Errorf("wrong number of parameters: got %d expected %d", len(a.Parameters), len(e.Parameters))
			}

			for ka, pa := range a.Parameters {
				pe, ok := e.Parameters[ka]
				if !ok {
					t.Errorf("parameter %s should not exist", ka)
				}
				if pa != pe {
					t.Errorf("wrong parameter: got %s expected %s", pa, pe)
				}
			}
		}

		if len(res.HeaderExtensions) != len(tc.expected.HeaderExtensions) {
			t.Errorf("wrong number of HeaderExtensions: got %d expected %d", len(res.HeaderExtensions), len(tc.expected.HeaderExtensions))
		}

		for i, a := range res.HeaderExtensions {
			e := tc.expected.HeaderExtensions[i]
			if a.ID != e.ID {
				t.Errorf("wrong ID: got %d expected %d", a.ID, e.ID)
			}
			if a.direction != e.direction {
				t.Errorf("wrong direction: got %s expected %s", a.direction, e.direction)
			}
			if a.URI != e.URI {
				t.Errorf("wrong URI: got %s expected %s", a.URI, e.URI)
			}

		}
	}
}

func TestParseSsrcMedia_Success(t *testing.T) {
	tt := []struct {
		in       sdp.Attribute
		expected sdpSSRCMedia
	}{
		{
			in: sdp.Attribute{
				Key:   "ssrc",
				Value: "3735928559 cname:something",
			},
			expected: sdpSSRCMedia{
				SSRC:      3735928559,
				Attribute: "cname",
				Value:     "something",
			},
		},
	}

	for _, tc := range tt {
		res, err := sdpParseSSRCMedia(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if res.SSRC != tc.expected.SSRC {
			t.Errorf("wrong ssrc: got %d expected %d", res.SSRC, tc.expected.SSRC)
		}
		if res.Attribute != tc.expected.Attribute {
			t.Errorf("wrong attribute: got %s expected %s", res.Attribute, tc.expected.Attribute)
		}
		if res.Value != tc.expected.Value {
			t.Errorf("wrong value: got %s expected %s", res.Value, tc.expected.Value)
		}
	}
}
