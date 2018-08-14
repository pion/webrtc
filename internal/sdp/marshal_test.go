package sdp

import (
	"testing"
	"net/url"
	"net"
)

const (
	CanonicalMarshalSDP = "v=0\n" +
	"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\n" +
	"s=SDP Seminar\n" +
	"i=A Seminar on the session description protocol\n" +
	"u=http://www.example.com/seminars/sdp.pdf\n" +
	"e=j.doe@example.com (Jane Doe)\n" +
	"p=+1 617 555-6011\n" +
	"c=IN IP4 224.2.17.12/127\n" +
	"b=X-YZ:128\n" +
	"b=AS:12345\n" +
	"t=2873397496 2873404696\n" +
	"t=3034423619 3042462419\n" +
	"r=604800 3600 0 90000\n" +
	"z=2882844526 -3600 2898848070 0\n" +
	"k=prompt\n" +
	"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\n" +
	"a=recvonly\n" +
	"m=audio 49170 RTP/AVP 0\n" +
	"i=Vivamus a posuere nisl\n" +
	"c=IN IP4 203.0.113.1\n" +
	"b=X-YZ:128\n" +
	"k=prompt\n" +
	"a=sendrecv\n" +
	"m=video 51372 RTP/AVP 99\n" +
	"a=rtpmap:99 h263-1998/90000\n"
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
		URI:                func() *url.URL { uri, _ := url.Parse("http://www.example.com/seminars/sdp.pdf"); return uri }(),
		EmailAddress:       &(&struct{ x EmailAddress }{"j.doe@example.com (Jane Doe)"}).x,
		PhoneNumber:        &(&struct{ x PhoneNumber }{"+1 617 555-6011"}).x,
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
				Type: "YZ",
				Bandwidth: 128,
			},
			{
				Type: "AS",
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
						Offsets: []int64{0, 90000},
					},
				},
			},
		},
		TimeZones: []TimeZone{
			{
				AdjustmentTime: 2882844526,
				Offset: -3600,
			},
			{
				AdjustmentTime: 2898848070,
				Offset: 0,
			},
		},
		EncryptionKey: &(&struct{ x EncryptionKey }{"prompt"}).x,
		Attributes: []Attribute{
			Attribute("candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host"),
			Attribute("recvonly"),
		},
		MediaDescriptions: []MediaDescription{
			{
				MediaName: MediaName{
					Media: "audio",
					Port: RangedPort{
						Value: 49170,
					},
					Protos: []string{"RTP", "AVP"},
					Formats: []int{0},
				},
				MediaTitle: &(&struct{ x Information }{"Vivamus a posuere nisl"}).x,
				ConnectionInformation: &ConnectionInformation{
					NetworkType: "IN",
					AddressType: "IP4",
					Address: &Address{
						IP:  net.ParseIP("203.0.113.1"),
					},
				},
				Bandwidth: []Bandwidth{
					{
						Experimental: true,
						Type: "YZ",
						Bandwidth: 128,
					},
				},
				EncryptionKey: &(&struct{ x EncryptionKey }{"prompt"}).x,
				Attributes: []Attribute{
					Attribute("sendrecv"),
				},
			},
			{
				MediaName: MediaName{
					Media: "video",
					Port: RangedPort{
						Value: 51372,
					},
					Protos: []string{"RTP", "AVP"},
					Formats: []int{99},
				},
				Attributes: []Attribute{
					Attribute("rtpmap:99 h263-1998/90000"),
				},
			},
		},
	}

	actual := sd.Marshal()
	if actual != CanonicalMarshalSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", CanonicalMarshalSDP, actual)
	}
}
