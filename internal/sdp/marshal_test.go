package sdp

import (
	"net"
	"net/url"
	"testing"
)

const (
	CanonicalMarshalSDP = "v=0\r\n" +
		"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
		"s=SDP Seminar\r\n" +
		"i=A Seminar on the session description protocol\r\n" +
		"u=http://www.example.com/seminars/sdp.pdf\r\n" +
		"e=j.doe@example.com (Jane Doe)\r\n" +
		"p=+1 617 555-6011\r\n" +
		"c=IN IP4 224.2.17.12/127\r\n" +
		"b=X-YZ:128\r\n" +
		"b=AS:12345\r\n" +
		"t=2873397496 2873404696\r\n" +
		"t=3034423619 3042462419\r\n" +
		"r=604800 3600 0 90000\r\n" +
		"z=2882844526 -3600 2898848070 0\r\n" +
		"k=prompt\r\n" +
		"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\r\n" +
		"a=recvonly\r\n" +
		"m=audio 49170 RTP/AVP 0\r\n" +
		"i=Vivamus a posuere nisl\r\n" +
		"c=IN IP4 203.0.113.1\r\n" +
		"b=X-YZ:128\r\n" +
		"k=prompt\r\n" +
		"a=sendrecv\r\n" +
		"m=video 51372 RTP/AVP 99\r\n" +
		"a=rtpmap:99 h263-1998/90000\r\n"
)

func TestMarshalCanonical(t *testing.T) {
	sd := &SessionDescription{
		Version: 0,
		Origin: Origin{
			Username:       "jdoe",
			SessionID:      uint64(2890844526),
			SessionVersion: uint64(2890842807),
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "10.47.16.5",
		},
		SessionName:        "SDP Seminar",
		SessionInformation: &(&struct{ x Information }{"A Seminar on the session description protocol"}).x,
		URI: func() *url.URL {
			uri, err := url.Parse("http://www.example.com/seminars/sdp.pdf")
			if err != nil {
				return nil
			}
			return uri
		}(),
		EmailAddress: &(&struct{ x EmailAddress }{"j.doe@example.com (Jane Doe)"}).x,
		PhoneNumber:  &(&struct{ x PhoneNumber }{"+1 617 555-6011"}).x,
		ConnectionInformation: &ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address: &Address{
				IP:  net.ParseIP("224.2.17.12"),
				TTL: &(&struct{ x int }{127}).x,
			},
		},
		Bandwidth: []Bandwidth{
			{
				Experimental: true,
				Type:         "YZ",
				Bandwidth:    128,
			},
			{
				Type:      "AS",
				Bandwidth: 12345,
			},
		},
		TimeDescriptions: []TimeDescription{
			{
				Timing: Timing{
					StartTime: 2873397496,
					StopTime:  2873404696,
				},
				RepeatTimes: nil,
			},
			{
				Timing: Timing{
					StartTime: 3034423619,
					StopTime:  3042462419,
				},
				RepeatTimes: []RepeatTime{
					{
						Interval: 604800,
						Duration: 3600,
						Offsets:  []int64{0, 90000},
					},
				},
			},
		},
		TimeZones: []TimeZone{
			{
				AdjustmentTime: 2882844526,
				Offset:         -3600,
			},
			{
				AdjustmentTime: 2898848070,
				Offset:         0,
			},
		},
		EncryptionKey: &(&struct{ x EncryptionKey }{"prompt"}).x,
		Attributes: []Attribute{
			NewAttribute("candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host", ""),
			NewAttribute("recvonly", ""),
		},
		MediaDescriptions: []*MediaDescription{
			{
				MediaName: MediaName{
					Media: "audio",
					Port: RangedPort{
						Value: 49170,
					},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
				MediaTitle: &(&struct{ x Information }{"Vivamus a posuere nisl"}).x,
				ConnectionInformation: &ConnectionInformation{
					NetworkType: "IN",
					AddressType: "IP4",
					Address: &Address{
						IP: net.ParseIP("203.0.113.1"),
					},
				},
				Bandwidth: []Bandwidth{
					{
						Experimental: true,
						Type:         "YZ",
						Bandwidth:    128,
					},
				},
				EncryptionKey: &(&struct{ x EncryptionKey }{"prompt"}).x,
				Attributes: []Attribute{
					NewAttribute("sendrecv", ""),
				},
			},
			{
				MediaName: MediaName{
					Media: "video",
					Port: RangedPort{
						Value: 51372,
					},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"99"},
				},
				Attributes: []Attribute{
					NewAttribute("rtpmap:99 h263-1998/90000", ""),
				},
			},
		},
	}

	actual := sd.Marshal()
	if actual != CanonicalMarshalSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", CanonicalMarshalSDP, actual)
	}
}
