// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package fmtp

import (
	"reflect"
	"testing"
)

func TestH264FMTPParse(t *testing.T) {
	testCases := map[string]struct {
		input    string
		expected FMTP
	}{
		"OneParam": {
			input: "key-name=value",
			expected: &h264FMTP{
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		"OneParamWithWhiteSpeces": {
			input: "\tkey-name=value ",
			expected: &h264FMTP{
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		"TwoParams": {
			input: "key-name=value;key2=value2",
			expected: &h264FMTP{
				parameters: map[string]string{
					"key-name": "value",
					"key2":     "value2",
				},
			},
		},
		"TwoParamsWithWhiteSpeces": {
			input: "key-name=value;  \n\tkey2=value2 ",
			expected: &h264FMTP{
				parameters: map[string]string{
					"key-name": "value",
					"key2":     "value2",
				},
			},
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			f := Parse("video/h264", testCase.input)
			if !reflect.DeepEqual(testCase.expected, f) {
				t.Errorf("Expected Fmtp params: %v, got: %v", testCase.expected, f)
			}

			if f.MimeType() != "video/h264" {
				t.Errorf("Expected MimeType of video/h264, got: %s", f.MimeType())
			}
		})
	}
}

func TestH264FMTPCompare(t *testing.T) {
	consistString := map[bool]string{true: "consist", false: "inconsist"}

	testCases := map[string]struct {
		a, b    string
		consist bool
	}{
		"Equal": {
			a:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			b:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			consist: true,
		},
		"EqualWithWhitespaceVariants": {
			a:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			b:       "  level-asymmetry-allowed=1;  \npacketization-mode=1;\t\nprofile-level-id=42e01f",
			consist: true,
		},
		"EqualWithCase": {
			a:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			b:       "level-asymmetry-allowed=1;packetization-mode=1;PROFILE-LEVEL-ID=42e01f",
			consist: true,
		},
		"OneHasExtraParam": {
			a:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			b:       "packetization-mode=1;profile-level-id=42e01f",
			consist: true,
		},
		"DifferentProfileLevelIDVersions": {
			a:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			b:       "packetization-mode=1;profile-level-id=42e029",
			consist: true,
		},
		"Inconsistent": {
			a:       "packetization-mode=1;profile-level-id=42e029",
			b:       "packetization-mode=0;profile-level-id=42e029",
			consist: false,
		},
		"Inconsistent_MissingPacketizationMode": {
			a:       "packetization-mode=1;profile-level-id=42e029",
			b:       "profile-level-id=42e029",
			consist: false,
		},
		"Inconsistent_MissingProfileLevelID": {
			a:       "packetization-mode=1;profile-level-id=42e029",
			b:       "packetization-mode=1",
			consist: false,
		},
		"Inconsistent_InvalidProfileLevelID": {
			a:       "packetization-mode=1;profile-level-id=42e029",
			b:       "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=41e029",
			consist: false,
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		check := func(t *testing.T, a, b string) {
			aa := Parse("video/h264", a)
			bb := Parse("video/h264", b)
			c := aa.Match(bb)
			if c != testCase.consist {
				t.Errorf(
					"'%s' and '%s' are expected to be %s, but treated as %s",
					a, b, consistString[testCase.consist], consistString[c],
				)
			}

			// test reverse case here
			c = bb.Match(aa)
			if c != testCase.consist {
				t.Errorf(
					"'%s' and '%s' are expected to be %s, but treated as %s",
					a, b, consistString[testCase.consist], consistString[c],
				)
			}
		}
		t.Run(name, func(t *testing.T) {
			check(t, testCase.a, testCase.b)
		})
	}
}
