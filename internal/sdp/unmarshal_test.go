package sdp

import (
	"testing"
)

const (
	BaseSDP = "v=0\n" +
		"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\n" +
		"s=SDP Seminar\n"

	SessionInformationSDP = BaseSDP +
		"i=A Seminar on the session description protocol\n" +
		"t=3034423619 3042462419\n"

	URISDP = BaseSDP +
		"u=http://www.example.com/seminars/sdp.pdf\n" +
		"t=3034423619 3042462419\n"

	EmailAddressSDP = BaseSDP +
		"e=j.doe@example.com (Jane Doe)\n" +
		"t=3034423619 3042462419\n"

	PhoneNumberSDP = BaseSDP +
		"p=+1 617 555-6011\n" +
		"t=3034423619 3042462419\n"

	SessionConnectionInformationSDP = BaseSDP +
		"c=IN IP4 224.2.17.12/127\n" +
		"t=3034423619 3042462419\n"

	SessionBandwidthSDP = BaseSDP +
		"b=X-YZ:128\n" +
		"b=AS:12345\n" +
		"t=3034423619 3042462419\n"

	TimingSDP = BaseSDP +
		"t=2873397496 2873404696\n"

	// Short hand time notation is converted into NTP timestamp format in
	// seconds. Because of that unittest comparisons will fail as the same time
	// will be expressed in different units.
	RepeatTimesSDP = TimingSDP +
		"r=604800 3600 0 90000\n" +
		"r=3d 2h 0 21h\n"

	RepeatTimesSDPExpected = TimingSDP +
		"r=604800 3600 0 90000\n" +
		"r=259200 7200 0 75600\n"

	// The expected value looks a bit different for the same reason as mentioned
	// above regarding RepeatTimes.
	TimeZonesSDP = TimingSDP +
		"r=2882844526 -1h 2898848070 0\n"

	TimeZonesSDPExpected = TimingSDP +
		"r=2882844526 -3600 2898848070 0\n"

	SessionEncryptionKeySDP = TimingSDP +
		"k=prompt\n"

	SessionAttributesSDP = TimingSDP +
		"a=rtpmap:96 opus/48000\n"

	MediaNameSDP = TimingSDP +
		"m=video 51372 RTP/AVP 99\n" +
		"m=audio 54400 RTP/SAVPF 0 96\n"

	MediaTitleSDP = MediaNameSDP +
		"i=Vivamus a posuere nisl\n"

	MediaConnectionInformationSDP = MediaNameSDP +
		"c=IN IP4 203.0.113.1\n"

	MediaBandwidthSDP = MediaNameSDP +
		"b=X-YZ:128\n" +
		"b=AS:12345\n"

	MediaEncryptionKeySDP = MediaNameSDP +
		"k=prompt\n"

	MediaAttributesSDP = MediaNameSDP +
		"a=rtpmap:99 h263-1998/90000\n" +
		"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\n"

	CanonicalUnmarshalSDP = "v=0\n" +
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

func TestUnmarshalSessionInformation(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(SessionInformationSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != SessionInformationSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", SessionInformationSDP, actual)
	}
}

func TestUnmarshalURI(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(URISDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != URISDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", URISDP, actual)
	}
}

func TestUnmarshalEmailAddress(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(EmailAddressSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != EmailAddressSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", EmailAddressSDP, actual)
	}
}

func TestUnmarshalPhoneNumber(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(PhoneNumberSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != PhoneNumberSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", PhoneNumberSDP, actual)
	}
}

func TestUnmarshalSessionConnectionInformation(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(SessionConnectionInformationSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != SessionConnectionInformationSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", SessionConnectionInformationSDP, actual)
	}
}

func TestUnmarshalSessionBandwidth(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(SessionBandwidthSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != SessionBandwidthSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", SessionBandwidthSDP, actual)
	}
}

func TestUnmarshalRepeatTimes(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(RepeatTimesSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != RepeatTimesSDPExpected {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", RepeatTimesSDPExpected, actual)
	}
}

func TestUnmarshalTimeZones(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(TimeZonesSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != TimeZonesSDPExpected {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", TimeZonesSDPExpected, actual)
	}
}

func TestUnmarshalSessionEncryptionKey(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(SessionEncryptionKeySDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != SessionEncryptionKeySDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", SessionEncryptionKeySDP, actual)
	}
}

func TestUnmarshalSessionAttributes(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(SessionAttributesSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != SessionAttributesSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", SessionAttributesSDP, actual)
	}
}

func TestUnmarshalMediaName(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(MediaNameSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != MediaNameSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", MediaNameSDP, actual)
	}
}

func TestUnmarshalMediaTitle(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(MediaTitleSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != MediaTitleSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", MediaTitleSDP, actual)
	}
}

func TestUnmarshalMediaConnectionInformation(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(MediaConnectionInformationSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != MediaConnectionInformationSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", MediaConnectionInformationSDP, actual)
	}
}

func TestUnmarshalMediaBandwidth(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(MediaBandwidthSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != MediaBandwidthSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", MediaBandwidthSDP, actual)
	}
}

func TestUnmarshalMediaEncryptionKey(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(MediaEncryptionKeySDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != MediaEncryptionKeySDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", MediaEncryptionKeySDP, actual)
	}
}

func TestUnmarshalMediaAttributes(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(MediaAttributesSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != MediaAttributesSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", MediaAttributesSDP, actual)
	}
}

func TestUnmarshalCanonical(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(CanonicalUnmarshalSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != CanonicalUnmarshalSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", CanonicalUnmarshalSDP, actual)
	}
}
