package webrtc

import (
	"testing"

	"github.com/pions/sdp/v2"
)

func TestCodecRegistration(t *testing.T) {
	const invalidPT = 255

	codecs := DefaultCodecs

	for _, test := range []struct {
		PayloadType uint8
		WantError   error
	}{
		{
			PayloadType: DefaultPayloadTypeG722,
			WantError:   nil,
		},
		{
			PayloadType: DefaultPayloadTypeOpus,
			WantError:   nil,
		},
		{
			PayloadType: DefaultPayloadTypeVP8,
			WantError:   nil,
		},
		{
			PayloadType: DefaultPayloadTypeVP9,
			WantError:   nil,
		},
		{
			PayloadType: DefaultPayloadTypeH264,
			WantError:   nil,
		},
		{
			PayloadType: invalidPT,
			WantError:   ErrCodecNotFound,
		},
	} {
		_, err := codecs.getCodec(test.PayloadType)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("getCodec(%v): err=%v, want %v", test.PayloadType, got, want)
		}
	}

	_, err := codecs.getCodecSDP(sdp.Codec{PayloadType: invalidPT})
	if got, want := err, ErrCodecNotFound; got != want {
		t.Fatalf("getCodecSDP(invalidPT): err=%v, want %v", got, want)
	}
}
