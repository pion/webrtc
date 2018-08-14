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

	UriSDP = BaseSDP +
	"u=http://www.example.com/seminars/sdp.pdf\n" +
	"t=3034423619 3042462419\n"

	TimingSDP = BaseSDP +
	"t=2873397496 2873404696\n"

	RepeatTimesSDP = TimingSDP +
	"r=604800 3600 0 90000\n"
)

const CanonicalSDP = "v=0\n" +
"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\n" +
"s=SDP Seminar\n" +
"i=A Seminar on the session description protocol\n" +
"u=http://www.example.com/seminars/sdp.pdf\n" +
"e=j.doe@example.com (Jane Doe)\n" +
"c=IN IP4 224.2.17.12/127\n" +
"b=X-YZ:128\n" +
"b=AS:12345\n" +
"t=2873397496 2873404696\n" +
"t=3034423619 3042462419\n" +
"r=604800 3600 0 90000\n" +
"z=2882844526 -1h 2898848070 0\n" +
"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\n" +
"a=candidate:1 2 UDP 2113667326 203.0.113.1 54401 typ host\n" +
"a=recvonly\n" +
"m=audio 49170 RTP/AVP 0\n" +
"m=video 51372 RTP/AVP 99\n" +
"a=rtpmap:99 h263-1998/90000\n"

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
	if err := sd.Unmarshal(UriSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != UriSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", UriSDP, actual)
	}
}

func TestUnmarshalRepeatTimes(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(RepeatTimesSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != RepeatTimesSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", RepeatTimesSDP, actual)
	}
}

func TestUnmarshalCanonical(t *testing.T) {
	sd := &SessionDescription{}
	if err := sd.Unmarshal(CanonicalSDP); err != nil {
		t.Errorf("error: %v", err)
	}

	actual := sd.Marshal()
	if actual != CanonicalSDP {
		t.Errorf("error:\n\nEXPECTED:\n%v\nACTUAL:\n%v", CanonicalSDP, actual)
	}
}
