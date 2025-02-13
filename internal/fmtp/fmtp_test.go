// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package fmtp

import (
	"reflect"
	"testing"
)

func TestParseParameters(t *testing.T) {
	for _, ca := range []struct {
		name       string
		line       string
		parameters map[string]string
	}{
		{
			"one param",
			"key-name=value",
			map[string]string{
				"key-name": "value",
			},
		},
		{
			"one param with white spaces",
			"\tkey-name=value ",
			map[string]string{
				"key-name": "value",
			},
		},
		{
			"two params",
			"key-name=value;key2=value2",
			map[string]string{
				"key-name": "value",
				"key2":     "value2",
			},
		},
		{
			"two params with white spaces",
			"key-name=value;  \n\tkey2=value2 ",
			map[string]string{
				"key-name": "value",
				"key2":     "value2",
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			parameters := parseParameters(ca.line)
			if !reflect.DeepEqual(parameters, ca.parameters) {
				t.Errorf("expected '%v', got '%v'", ca.parameters, parameters)
			}
		})
	}
}

func TestParse(t *testing.T) {
	for _, ca := range []struct {
		name      string
		mimeType  string
		clockRate uint32
		channels  uint16
		line      string
		expected  FMTP
	}{
		{
			"generic",
			"generic",
			90000,
			2,
			"key-name=value",
			&genericFMTP{
				mimeType:  "generic",
				clockRate: 90000,
				channels:  2,
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		{
			"generic case normalization",
			"generic",
			90000,
			2,
			"Key=value",
			&genericFMTP{
				mimeType:  "generic",
				clockRate: 90000,
				channels:  2,
				parameters: map[string]string{
					"key": "value",
				},
			},
		},
		{
			"h264",
			"video/h264",
			90000,
			0,
			"key-name=value",
			&h264FMTP{
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		{
			"vp9",
			"video/vp9",
			90000,
			0,
			"key-name=value",
			&vp9FMTP{
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		{
			"av1",
			"video/av1",
			90000,
			0,
			"key-name=value",
			&av1FMTP{
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			f := Parse(ca.mimeType, ca.clockRate, ca.channels, ca.line)
			if !reflect.DeepEqual(ca.expected, f) {
				t.Errorf("expected '%v', got '%v'", ca.expected, f)
			}

			if f.MimeType() != ca.mimeType {
				t.Errorf("Expected '%v', got '%s'", ca.mimeType, f.MimeType())
			}
		})
	}
}

func TestMatch(t *testing.T) { //nolint:maintidx
	consistString := map[bool]string{true: "consist", false: "inconsist"}

	for _, ca := range []struct {
		name    string
		a       FMTP
		b       FMTP
		consist bool
	}{
		{
			"generic equal",
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			true,
		},
		{
			"generic one extra param",
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
					"key4": "value4",
				},
			},
			true,
		},
		{
			"generic inferred channels",
			&genericFMTP{
				mimeType: "generic",
				channels: 1,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			true,
		},
		{
			"generic inconsistent different kind",
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&h264FMTP{},
			false,
		},
		{
			"generic inconsistent different mime type",
			&genericFMTP{
				mimeType: "generic1",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType: "generic2",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			false,
		},
		{
			"generic inconsistent different clock rate",
			&genericFMTP{
				mimeType:  "generic",
				clockRate: 90000,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "generic",
				clockRate: 48000,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			false,
		},
		{
			"generic inconsistent different channels",
			&genericFMTP{
				mimeType:  "generic",
				clockRate: 90000,
				channels:  2,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "generic",
				clockRate: 90000,
				channels:  1,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			false,
		},
		{
			"generic inconsistent different parameters",
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key1": "value1",
					"key2": "different_value",
					"key3": "value3",
				},
			},
			false,
		},
		{
			"h264 equal",
			&h264FMTP{
				parameters: map[string]string{
					"level-asymmetry-allowed": "1",
					"packetization-mode":      "1",
					"profile-level-id":        "42e01f",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"level-asymmetry-allowed": "1",
					"packetization-mode":      "1",
					"profile-level-id":        "42e01f",
				},
			},
			true,
		},
		{
			"h264 one extra param",
			&h264FMTP{
				parameters: map[string]string{
					"level-asymmetry-allowed": "1",
					"packetization-mode":      "1",
					"profile-level-id":        "42e01f",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "42e01f",
				},
			},
			true,
		},
		{
			"h264 different profile level ids version",
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "42e01f",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "42e029",
				},
			},
			true,
		},
		{
			"h264 inconsistent different kind",
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "0",
					"profile-level-id":   "42e01f",
				},
			},
			&genericFMTP{},
			false,
		},
		{
			"h264 inconsistent different parameters",
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "0",
					"profile-level-id":   "42e01f",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "42e01f",
				},
			},
			false,
		},
		{
			"h264 inconsistent missing packetization mode",
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "0",
					"profile-level-id":   "42e01f",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"profile-level-id": "42e01f",
				},
			},
			false,
		},
		{
			"h264 inconsistent missing profile level id",
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "42e01f",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
				},
			},
			false,
		},
		{
			"h264 inconsistent invalid profile level id",
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "42e029",
				},
			},
			&h264FMTP{
				parameters: map[string]string{
					"packetization-mode": "1",
					"profile-level-id":   "41e029",
				},
			},
			false,
		},
		{
			"vp9 equal",
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "1",
				},
			},
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "1",
				},
			},
			true,
		},
		{
			"vp9 missing profile",
			&vp9FMTP{
				parameters: map[string]string{},
			},
			&vp9FMTP{
				parameters: map[string]string{},
			},
			true,
		},
		{
			"vp9 inferred profile",
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "0",
				},
			},
			&vp9FMTP{
				parameters: map[string]string{},
			},
			true,
		},
		{
			"vp9 inconsistent different kind",
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "0",
				},
			},
			&genericFMTP{},
			false,
		},
		{
			"vp9 inconsistent different profile",
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "0",
				},
			},
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "1",
				},
			},
			false,
		},
		{
			"vp9 inconsistent different inferred profile",
			&vp9FMTP{
				parameters: map[string]string{},
			},
			&vp9FMTP{
				parameters: map[string]string{
					"profile-id": "1",
				},
			},
			false,
		},
		{
			"av1 equal",
			&av1FMTP{
				parameters: map[string]string{
					"profile": "1",
				},
			},
			&av1FMTP{
				parameters: map[string]string{
					"profile": "1",
				},
			},
			true,
		},
		{
			"av1 missing profile",
			&av1FMTP{
				parameters: map[string]string{},
			},
			&av1FMTP{
				parameters: map[string]string{},
			},
			true,
		},
		{
			"av1 inferred profile",
			&av1FMTP{
				parameters: map[string]string{
					"profile": "0",
				},
			},
			&av1FMTP{
				parameters: map[string]string{},
			},
			true,
		},
		{
			"av1 inconsistent different kind",
			&av1FMTP{
				parameters: map[string]string{
					"profile": "0",
				},
			},
			&genericFMTP{},
			false,
		},
		{
			"av1 inconsistent different profile",
			&av1FMTP{
				parameters: map[string]string{
					"profile": "0",
				},
			},
			&av1FMTP{
				parameters: map[string]string{
					"profile": "1",
				},
			},
			false,
		},
		{
			"av1 inconsistent different inferred profile",
			&av1FMTP{
				parameters: map[string]string{},
			},
			&av1FMTP{
				parameters: map[string]string{
					"profile": "1",
				},
			},
			false,
		},
		{
			"pcmu channels",
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 8000,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 8000,
				channels:  1,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			true,
		},
		{
			"pcmu inconsistent channels",
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 8000,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 8000,
				channels:  2,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			false,
		},
		{
			"pcmu clockrate",
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 0,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 8000,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			true,
		},
		{
			"pcmu inconsistent clockrate",
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 0,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "audio/pcmu",
				clockRate: 16000,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			false,
		},
		{
			"opus clockrate",
			&genericFMTP{
				mimeType:  "audio/opus",
				clockRate: 0,
				channels:  0,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			&genericFMTP{
				mimeType:  "audio/opus",
				clockRate: 48000,
				channels:  2,
				parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			true,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			c := ca.a.Match(ca.b)
			if c != ca.consist {
				t.Errorf(
					"'%s' and '%s' are expected to be %s, but treated as %s",
					ca.a, ca.b, consistString[ca.consist], consistString[c],
				)
			}

			c = ca.b.Match(ca.a)
			if c != ca.consist {
				t.Errorf(
					"'%s' and '%s' are expected to be %s, but treated as %s",
					ca.a, ca.b, consistString[ca.consist], consistString[c],
				)
			}
		})
	}
}
