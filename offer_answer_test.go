package webrtc

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/pion/sdp/v2"
)

// Typical offer sent by Safari
const offerJSON = "{\"type\":\"offer\",\"sdp\":\"v=0\\r\\no=- 3551982888046034593 2 IN IP4 127.0.0.1\\r\\ns=-\\r\\nt=0 0\\r\\na=group:BUNDLE 0 1\\r\\na=msid-semantic: WMS 988dbe9a-f0dd-4c80-9445-40bcbec8132b\\r\\nm=video 49691 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 127 125 104\\r\\nc=IN IP4 73.164.206.100\\r\\na=rtcp:9 IN IP4 0.0.0.0\\r\\na=candidate:3405893845 1 udp 2113937151 10.0.1.22 49691 typ host generation 0 network-cost 999\\r\\na=candidate:842163049 1 udp 1677729535 73.164.206.100 49691 typ srflx raddr 10.0.1.22 rport 49691 generation 0 network-cost 999\\r\\na=ice-ufrag:17df\\r\\na=ice-pwd:IUohGJRpNgFi0H5ryr5To2G9\\r\\na=ice-options:trickle\\r\\na=fingerprint:sha-256 1C:B4:37:B1:B4:4B:85:DE:C6:37:C5:F1:D0:60:F3:BD:E3:6C:A0:CA:A9:9D:23:8B:3F:5D:92:54:1B:0A:0A:4F\\r\\na=setup:actpass\\r\\na=mid:0\\r\\na=extmap:2 urn:ietf:params:rtp-hdrext:toffset\\r\\na=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\\r\\na=extmap:4 urn:3gpp:video-orientation\\r\\na=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\\r\\na=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay\\r\\na=extmap:7 http://www.webrtc.org/experiments/rtp-hdrext/video-content-type\\r\\na=extmap:8 http://www.webrtc.org/experiments/rtp-hdrext/video-timing\\r\\na=extmap:10 http://tools.ietf.org/html/draft-ietf-avtext-framemarking-07\\r\\na=extmap:9 urn:ietf:params:rtp-hdrext:sdes:mid\\r\\na=sendrecv\\r\\na=msid:988dbe9a-f0dd-4c80-9445-40bcbec8132b 3c7514ef-fd3b-454f-a89d-91c3daf7940b\\r\\na=rtcp-mux\\r\\na=rtcp-rsize\\r\\na=rtpmap:96 H264/90000\\r\\na=rtcp-fb:96 goog-remb\\r\\na=rtcp-fb:96 transport-cc\\r\\na=rtcp-fb:96 ccm fir\\r\\na=rtcp-fb:96 nack\\r\\na=rtcp-fb:96 nack pli\\r\\na=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640c1f\\r\\na=rtpmap:97 rtx/90000\\r\\na=fmtp:97 apt=96\\r\\na=rtpmap:98 H264/90000\\r\\na=rtcp-fb:98 goog-remb\\r\\na=rtcp-fb:98 transport-cc\\r\\na=rtcp-fb:98 ccm fir\\r\\na=rtcp-fb:98 nack\\r\\na=rtcp-fb:98 nack pli\\r\\na=fmtp:98 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\\r\\na=rtpmap:99 rtx/90000\\r\\na=fmtp:99 apt=98\\r\\na=rtpmap:100 VP8/90000\\r\\na=rtcp-fb:100 goog-remb\\r\\na=rtcp-fb:100 transport-cc\\r\\na=rtcp-fb:100 ccm fir\\r\\na=rtcp-fb:100 nack\\r\\na=rtcp-fb:100 nack pli\\r\\na=rtpmap:101 rtx/90000\\r\\na=fmtp:101 apt=100\\r\\na=rtpmap:127 red/90000\\r\\na=rtpmap:125 rtx/90000\\r\\na=fmtp:125 apt=127\\r\\na=rtpmap:104 ulpfec/90000\\r\\na=ssrc-group:FID 1673692565 1797115165\\r\\na=ssrc:1673692565 cname:y1xYBMcU+DF2yI0q\\r\\na=ssrc:1673692565 msid:988dbe9a-f0dd-4c80-9445-40bcbec8132b 3c7514ef-fd3b-454f-a89d-91c3daf7940b\\r\\na=ssrc:1673692565 mslabel:988dbe9a-f0dd-4c80-9445-40bcbec8132b\\r\\na=ssrc:1673692565 label:3c7514ef-fd3b-454f-a89d-91c3daf7940b\\r\\na=ssrc:1797115165 cname:y1xYBMcU+DF2yI0q\\r\\na=ssrc:1797115165 msid:988dbe9a-f0dd-4c80-9445-40bcbec8132b 3c7514ef-fd3b-454f-a89d-91c3daf7940b\\r\\na=ssrc:1797115165 mslabel:988dbe9a-f0dd-4c80-9445-40bcbec8132b\\r\\na=ssrc:1797115165 label:3c7514ef-fd3b-454f-a89d-91c3daf7940b\\r\\nm=audio 57736 UDP/TLS/RTP/SAVPF 111 103 9 102 0 8 105 13 110 113 126\\r\\nc=IN IP4 73.164.206.100\\r\\na=rtcp:9 IN IP4 0.0.0.0\\r\\na=candidate:3405893845 1 udp 2113937151 10.0.1.22 57736 typ host generation 0 network-cost 999\\r\\na=candidate:842163049 1 udp 1677729535 73.164.206.100 57736 typ srflx raddr 10.0.1.22 rport 57736 generation 0 network-cost 999\\r\\na=ice-ufrag:17df\\r\\na=ice-pwd:IUohGJRpNgFi0H5ryr5To2G9\\r\\na=ice-options:trickle\\r\\na=fingerprint:sha-256 1C:B4:37:B1:B4:4B:85:DE:C6:37:C5:F1:D0:60:F3:BD:E3:6C:A0:CA:A9:9D:23:8B:3F:5D:92:54:1B:0A:0A:4F\\r\\na=setup:actpass\\r\\na=mid:1\\r\\na=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level\\r\\na=extmap:9 urn:ietf:params:rtp-hdrext:sdes:mid\\r\\na=sendrecv\\r\\na=msid:988dbe9a-f0dd-4c80-9445-40bcbec8132b 8daa4cd6-d9a8-4fc4-bba5-1ac6738f6e9e\\r\\na=rtcp-mux\\r\\na=rtpmap:111 opus/48000/2\\r\\na=rtcp-fb:111 transport-cc\\r\\na=fmtp:111 minptime=10;useinbandfec=1\\r\\na=rtpmap:103 ISAC/16000\\r\\na=rtpmap:9 G722/8000\\r\\na=rtpmap:102 ILBC/8000\\r\\na=rtpmap:0 PCMU/8000\\r\\na=rtpmap:8 PCMA/8000\\r\\na=rtpmap:105 CN/16000\\r\\na=rtpmap:13 CN/8000\\r\\na=rtpmap:110 telephone-event/48000\\r\\na=rtpmap:113 telephone-event/16000\\r\\na=rtpmap:126 telephone-event/8000\\r\\na=ssrc:1559770673 cname:y1xYBMcU+DF2yI0q\\r\\na=ssrc:1559770673 msid:988dbe9a-f0dd-4c80-9445-40bcbec8132b 8daa4cd6-d9a8-4fc4-bba5-1ac6738f6e9e\\r\\na=ssrc:1559770673 mslabel:988dbe9a-f0dd-4c80-9445-40bcbec8132b\\r\\na=ssrc:1559770673 label:8daa4cd6-d9a8-4fc4-bba5-1ac6738f6e9e\\r\\n\"}"

// TestAnswerUsingOfferCodecs ensures that the answer contains payload types and codec formats
// from the offer instead of locally generated ones.
// The logic here is the same as from examples/echo, so if this test fails it likely means that
// examples/echo also needs attention.
func TestAnswerUsingOfferCodecs(t *testing.T) {
	api := NewAPI()
	// Set up a peer connection with video and audio transceivers
	// using defaults for all media
	config := Configuration{}
	peerConnection, err := api.NewPeerConnection(config)
	check(err)
	// Add codecs mentioned in the offer
	// Choose offer sdp from Safari, which the default answer doesn't handle properly
	offer := SessionDescription{}
	err = json.Unmarshal([]byte(offerJSON), &offer)
	check(err)
	var offerSD sdp.SessionDescription
	err = offerSD.Unmarshal([]byte(offer.SDP))
	check(err)
	for _, md := range offerSD.MediaDescriptions {
		formats, err := md.MediaFormats()
		check(err)
		for _, format := range formats {
			if format.MediaType == "video" || format.MediaType == "audio" {
				splits := strings.Split(format.EncodingName, "/")
				if len(splits) < 2 {
					t.Fatalf("unexpected encoding name %s", format.EncodingName)
				}
				codecName := splits[0]
				cr, err := strconv.Atoi(splits[1])
				if err != nil {
					t.Fatalf("couldn't extract integer clock rate from encoding name %s", format.EncodingName)
				}
				clockRate := uint32(cr)
				payloadType := uint8(format.PayloadType)
				var codec *RTPCodec
				switch codecName {
				case G722:
					codec = NewRTPG722Codec(payloadType, clockRate)
				case Opus:
					codec = NewRTPOpusCodec(payloadType, clockRate)
				case VP8:
					codec = NewRTPVP8Codec(payloadType, clockRate)
				case VP9:
					codec = NewRTPVP9Codec(payloadType, clockRate)
				case H264:
					codec = NewRTPH264Codec(payloadType, clockRate)
					codec.SDPFmtpLine = format.Parameters
				default:
					//t.Logf("ignoring offer codec %s", codecName)
					continue
				}
				api.mediaEngine.RegisterCodec(codec)
			}
		}
	}
	_, err = peerConnection.AddTransceiverFromKind(RTPCodecTypeVideo)
	check(err)
	_, err = peerConnection.AddTransceiverFromKind(RTPCodecTypeAudio)
	check(err)
	err = peerConnection.SetRemoteDescription(offer)
	check(err)
	answer, err := peerConnection.CreateAnswer(nil)
	check(err)
	var answerSD sdp.SessionDescription
	err = answerSD.Unmarshal([]byte(answer.SDP))
	check(err)
	if len(answerSD.MediaDescriptions) != 2 {
		t.Fatal("expected two media sections in answer, but got ", len(answerSD.MediaDescriptions))
	}
	// These formats are taken from the offer SDP.
	videoFormats := []*sdp.MediaFormat{
		&sdp.MediaFormat{MediaType: "video", PayloadType: 96, EncodingName: "H264/90000", Parameters: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640c1f"},
		&sdp.MediaFormat{MediaType: "video", PayloadType: 98, EncodingName: "H264/90000", Parameters: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"},
		&sdp.MediaFormat{MediaType: "video", PayloadType: 100, EncodingName: "VP8/90000"}}
	audioFormats := []*sdp.MediaFormat{
		&sdp.MediaFormat{MediaType: "audio", PayloadType: 111, EncodingName: "opus/48000/2", Parameters: "minptime=10;useinbandfec=1"},
		&sdp.MediaFormat{MediaType: "audio", PayloadType: 9, EncodingName: "G722/8000"}}
	// Compare video answers in order
	answerVideoFormats, err := answerSD.MediaDescriptions[0].MediaFormats()
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range answerVideoFormats {
		if !f.SameFormat(videoFormats[i]) {
			t.Fatalf("unexpected answer video format: %+v", f)
		}
	}
	// Compare audio answers in order
	answerAudioFormats, err := answerSD.MediaDescriptions[1].MediaFormats()
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range answerAudioFormats {
		if !f.SameFormat(audioFormats[i]) {
			t.Fatalf("unexpected answer audio format: %+v", f)
		}
	}
}
