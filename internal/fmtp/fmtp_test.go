// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package fmtp

import (
	"reflect"
	"testing"
)

func TestGenericParseFmtp(t *testing.T) {
	testCases := map[string]struct {
		input    string
		expected FMTP
	}{
		"OneParam": {
			input: "key-name=value",
			expected: &genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		"OneParamWithWhiteSpeces": {
			input: "\tkey-name=value ",
			expected: &genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key-name": "value",
				},
			},
		},
		"TwoParams": {
			input: "key-name=value;key2=value2",
			expected: &genericFMTP{
				mimeType: "generic",
				parameters: map[string]string{
					"key-name": "value",
					"key2":     "value2",
				},
			},
		},
		"TwoParamsWithWhiteSpeces": {
			input: "key-name=value;  \n\tkey2=value2 ",
			expected: &genericFMTP{
				mimeType: "generic",
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
			f := Parse("generic", testCase.input)
			if !reflect.DeepEqual(testCase.expected, f) {
				t.Errorf("Expected Fmtp params: %v, got: %v", testCase.expected, f)
			}

			if f.MimeType() != "generic" {
				t.Errorf("Expected MimeType of generic, got: %s", f.MimeType())
			}
		})
	}
}

func TestGenericFmtpCompare(t *testing.T) {
	consistString := map[bool]string{true: "consist", false: "inconsist"}

	testCases := map[string]struct {
		a, b    string
		consist bool
	}{
		"Equal": {
			a:       "key1=value1;key2=value2;key3=value3",
			b:       "key1=value1;key2=value2;key3=value3",
			consist: true,
		},
		"EqualWithWhitespaceVariants": {
			a:       "key1=value1;key2=value2;key3=value3",
			b:       "  key1=value1;  \nkey2=value2;\t\nkey3=value3",
			consist: true,
		},
		"EqualWithCase": {
			a:       "key1=value1;key2=value2;key3=value3",
			b:       "key1=value1;key2=Value2;Key3=value3",
			consist: true,
		},
		"OneHasExtraParam": {
			a:       "key1=value1;key2=value2;key3=value3",
			b:       "key1=value1;key2=value2;key3=value3;key4=value4",
			consist: true,
		},
		"Inconsistent": {
			a:       "key1=value1;key2=value2;key3=value3",
			b:       "key1=value1;key2=different_value;key3=value3",
			consist: false,
		},
		"Inconsistent_OneHasExtraParam": {
			a:       "key1=value1;key2=value2;key3=value3;key4=value4",
			b:       "key1=value1;key2=different_value;key3=value3",
			consist: false,
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		check := func(t *testing.T, a, b string) {
			aa := Parse("", a)
			bb := Parse("", b)
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
