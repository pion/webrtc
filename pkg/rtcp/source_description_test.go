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
			Name: "nil",
			Data: nil,
			Want: SourceDescription{
				Chunks: nil,
			},
		},
		{
			Name: "missing type",
			Data: []byte{
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
			},
			WantError: errPacketTooShort,
		},
		{
			Name: "bad cname length",
			Data: []byte{
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
				// ssrc=0x00000000
				0x00, 0x00, 0x00, 0x00,
				// CNAME, len=1, content=A
				0x01, 0x02, 0x41,
				// Missing END
			},
			WantError: errPacketTooShort,
		},
		{
			Name:      "bad octet count",
			Data:      []byte{0, 0, 0, 0, 1, 1},
			WantError: errPacketTooShort,
		},
		{
			Name: "zero item chunk",
			Data: []byte{
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
			Name: "empty string",
			Data: []byte{
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
						Text: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					}},
				}},
			},
			WantError: errSDESTextTooLong,
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
