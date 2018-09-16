package rtcp

import (
	"reflect"
	"testing"
)

func TestSourceDescriptionUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      SourceDescription
		WantError error
	}{
		{
			Name:      "nil",
			Data:      nil,
			WantError: errInvalidHeader,
		},
		{
			Name: "no chunks",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=8
				0x80, 0xca, 0x00, 0x04,
			},
			Want: SourceDescription{
				Chunks: nil,
			},
		},
		{
			Name: "missing type",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=8
				0x81, 0xca, 0x00, 0x08,
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "bad cname length",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=10
				0x81, 0xca, 0x00, 0x0a,
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
				// CNAME, len = 1
				0x01, 0x01,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "short cname",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=9
				0x81, 0xca, 0x00, 0x09,
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
				// CNAME, Missing length
				0x01,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "no end",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=11
				0x81, 0xca, 0x00, 0x0b,
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
				// CNAME, len=1, content=A
				0x01, 0x02, 0x41,
				// Missing END
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "bad octet count",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=10
				0x81, 0xca, 0x00, 0x0a,
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
				// CNAME, len=1
				0x01, 0x01,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "zero item chunk",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=12
				0x81, 0xca, 0x00, 0x0c,
				// ssrc=0x01020304
				0x01, 0x02, 0x03, 0x04,
				// END + padding
				0x00, 0x00, 0x00, 0x00,
			},
			Want: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Source: 0x01020304,
					Items:  nil,
				}},
			},
		},
		{
			Name: "wrong type",
			Data: []byte{
				// v=2, p=0, count=1, SR, len=12
				0x81, 0xc8, 0x00, 0x0c,
				// ssrc=0x01020304
				0x01, 0x02, 0x03, 0x04,
				// END + padding
				0x00, 0x00, 0x00, 0x00,
			},
			WantError: errWrongType,
		},
		{
			Name: "bad count in header",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=12
				0x81, 0xca, 0x00, 0x0c,
			},
			WantError: errInvalidHeader,
		},
		{
			Name: "empty string",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=12
				0x81, 0xca, 0x00, 0x0c,
				// ssrc=0x01020304
				0x01, 0x02, 0x03, 0x04,
				// CNAME, len=0
				0x01, 0x00,
				// END + padding
				0x00, 0x00,
			},
			Want: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Source: 0x01020304,
					Items: []SourceDescriptionItem{
						{
							Type: SDESCNAME,
							Text: "",
						},
					},
				}},
			},
		},
		{
			Name: "two items",
			Data: []byte{
				// v=2, p=0, count=1, SDES, len=16
				0x81, 0xca, 0x00, 0x10,
				// ssrc=0x10000000
				0x10, 0x00, 0x00, 0x00,
				// CNAME, len=1, content=A
				0x01, 0x01, 0x41,
				// PHONE, len=1, content=B
				0x04, 0x01, 0x42,
				// END + padding
				0x00, 0x00,
			},
			Want: SourceDescription{
				Chunks: []SourceDescriptionChunk{
					{
						Source: 0x10000000,
						Items: []SourceDescriptionItem{
							{
								Type: SDESCNAME,
								Text: "A",
							},
							{
								Type: SDESPhone,
								Text: "B",
							},
						},
					},
				},
			},
		},
		{
			Name: "two chunks",
			Data: []byte{
				// v=2, p=0, count=2, SDES, len=24
				0x82, 0xca, 0x00, 0x18,
				// ssrc=0x01020304
				0x01, 0x02, 0x03, 0x04,
				// Chunk 1
				// CNAME, len=1, content=A
				0x01, 0x01, 0x41,
				// END
				0x00,
				// Chunk 2
				// SSRC 0x05060708
				0x05, 0x06, 0x07, 0x08,
				// CNAME, len=3, content=BCD
				0x01, 0x03, 0x42, 0x43, 0x44,
				// END
				0x00, 0x00, 0x00,
			},
			Want: SourceDescription{
				Chunks: []SourceDescriptionChunk{
					{
						Source: 0x01020304,
						Items: []SourceDescriptionItem{
							{
								Type: SDESCNAME,
								Text: "A",
							},
						},
					},
					{
						Source: 0x05060708,
						Items: []SourceDescriptionItem{
							{
								Type: SDESCNAME,
								Text: "BCD",
							},
						},
					},
				},
			},
		},
	} {
		var sdes SourceDescription
		err := sdes.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := sdes, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q: got %#v, want %#v", test.Name, got, want)
		}
	}
}

func TestSourceDescriptionRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Desc      SourceDescription
		WantError error
	}{
		{
			Name: "valid",
			Desc: SourceDescription{
				Chunks: []SourceDescriptionChunk{
					{
						Source: 1,
						Items: []SourceDescriptionItem{
							{
								Type: SDESCNAME,
								Text: "test@example.com",
							},
						},
					},
					{
						Source: 2,
						Items: []SourceDescriptionItem{
							{
								Type: SDESNote,
								Text: "some note",
							},
							{
								Type: SDESNote,
								Text: "another note",
							},
						},
					},
				},
			},
		},
		{
			Name: "item without type",
			Desc: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Source: 1,
					Items: []SourceDescriptionItem{{
						Text: "test@example.com",
					}},
				}},
			},
			WantError: errSDESMissingType,
		},
		{
			Name: "zero items",
			Desc: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Source: 1,
				}},
			},
		},
		{
			Name: "email item",
			Desc: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Source: 1,
					Items: []SourceDescriptionItem{{
						Type: SDESEmail,
						Text: "test@example.com",
					}},
				}},
			},
		},
		{
			Name: "empty text",
			Desc: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Source: 1,
					Items: []SourceDescriptionItem{{
						Type: SDESCNAME,
						Text: "",
					}},
				}},
			},
		},
		{
			Name: "text too long",
			Desc: SourceDescription{
				Chunks: []SourceDescriptionChunk{{
					Items: []SourceDescriptionItem{{
						Type: SDESCNAME,
						Text: tooLongText,
					}},
				}},
			},
			WantError: errSDESTextTooLong,
		},
		{
			Name: "count overflow",
			Desc: SourceDescription{
				Chunks: tooManyChunks,
			},
			WantError: errTooManyChunks,
		},
	} {
		data, err := test.Desc.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded SourceDescription
		if err := decoded.Unmarshal(data); err != nil {
			t.Fatalf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Desc; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q sdes round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}

// a slice with enough SourceDescriptionChunks to overflow an 5-bit int
var tooManyChunks []SourceDescriptionChunk
var tooLongText string

func init() {
	for i := 0; i < (1 << 5); i++ {
		tooManyChunks = append(tooManyChunks, SourceDescriptionChunk{})
	}
	for i := 0; i < (1 << 8); i++ {
		tooLongText += "x"
	}
}
