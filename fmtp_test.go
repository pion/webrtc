package webrtc

import (
	"reflect"
	"testing"
)

func TestParseFmtp(t *testing.T) {
	testCases := map[string]struct {
		input    string
		expected fmtp
	}{
		"OneParam": {
			input: "key-name=value",
			expected: fmtp{
				"key-name": "value",
			},
		},
		"OneParamWithWhiteSpeces": {
			input: "\tkey-name=value ",
			expected: fmtp{
				"key-name": "value",
			},
		},
		"TwoParams": {
			input: "key-name=value;key2=value2",
			expected: fmtp{
				"key-name": "value",
				"key2":     "value2",
			},
		},
		"TwoParamsWithWhiteSpeces": {
			input: "key-name=value;  \n\tkey2=value2 ",
			expected: fmtp{
				"key-name": "value",
				"key2":     "value2",
			},
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			f := parseFmtp(testCase.input)
			if !reflect.DeepEqual(testCase.expected, f) {
				t.Errorf("Expected Fmtp params: %v, got: %v", testCase.expected, f)
			}
		})
	}
}

func TestFmtpConsist(t *testing.T) {
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
			c := fmtpConsist(parseFmtp(a), parseFmtp(b))
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
		t.Run(name+"_Reversed", func(t *testing.T) {
			check(t, testCase.b, testCase.a)
		})
	}
}
