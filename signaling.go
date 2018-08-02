package webrtc

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/sdp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pkg/errors"
)

/*
                      setRemote(OFFER)               setLocal(PRANSWER)
                          /-----\                               /-----\
                          |     |                               |     |
                          v     |                               v     |
           +---------------+    |                +---------------+    |
           |               |----/                |               |----/
           |  have-        | setLocal(PRANSWER)  | have-         |
           |  remote-offer |------------------- >| local-pranswer|
           |               |                     |               |
           |               |                     |               |
           +---------------+                     +---------------+
                ^   |                                   |
                |   | setLocal(ANSWER)                  |
  setRemote(OFFER)  |                                   |
                |   V                  setLocal(ANSWER) |
           +---------------+                            |
           |               |                            |
           |               |<---------------------------+
           |    stable     |
           |               |<---------------------------+
           |               |                            |
           +---------------+          setRemote(ANSWER) |
                ^   |                                   |
                |   | setLocal(OFFER)                   |
  setRemote(ANSWER) |                                   |
                |   V                                   |
           +---------------+                     +---------------+
           |               |                     |               |
           |  have-        | setRemote(PRANSWER) |have-          |
           |  local-offer  |------------------- >|remote-pranswer|
           |               |                     |               |
           |               |----\                |               |----\
           +---------------+    |                +---------------+    |
                          ^     |                               ^     |
                          |     |                               |     |
                          \-----/                               \-----/
                      setLocal(OFFER)               setRemote(PRANSWER)
*/

// RTCSignalingState indicates the state of the offer/answer process
type RTCSignalingState int

const (

	// RTCSignalingStateStable indicates there is no offer­answer exchange in progress.
	RTCSignalingStateStable RTCSignalingState = iota + 1

	// RTCSignalingStateHaveLocalOffer indicates A local description, of type "offer", has been successfully applied.
	RTCSignalingStateHaveLocalOffer

	// RTCSignalingStateHaveRemoteOffer indicates A remote description, of type "offer", has been successfully applied.
	RTCSignalingStateHaveRemoteOffer

	// RTCSignalingStateHaveLocalPranswer indicates A remote description of type "offer" has been successfully applied and a local description of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveLocalPranswer

	// RTCSignalingStateHaveRemotePranswer indicates A local description of type "offer" has been successfully applied and a remote description of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveRemotePranswer

	// RTCSignalingStateClosed indicates The RTCPeerConnection has been closed.
	RTCSignalingStateClosed
)

func (t RTCSignalingState) String() string {
	switch t {
	case RTCSignalingStateStable:
		return "stable"
	case RTCSignalingStateHaveLocalOffer:
		return "have-local-offer"
	case RTCSignalingStateHaveRemoteOffer:
		return "have-remote-offer"
	case RTCSignalingStateHaveLocalPranswer:
		return "have-local-pranswer"
	case RTCSignalingStateHaveRemotePranswer:
		return "have-remote-pranswer"
	case RTCSignalingStateClosed:
		return "closed"
	default:
		return "Unknown"
	}
}

// RTCSdpType describes the type of an RTCSessionDescription
type RTCSdpType int

const (

	// RTCSdpTypeOffer indicates that a description MUST be treated as an SDP offer.
	RTCSdpTypeOffer RTCSdpType = iota + 1

	// RTCSdpTypePranswer indicates that a description MUST be treated as an SDP answer, but not a final answer.
	RTCSdpTypePranswer

	// RTCSdpTypeAnswer indicates that a description MUST be treated as an SDP final answer, and the offer-answer exchange MUST be considered complete.
	RTCSdpTypeAnswer

	// RTCSdpTypeRollback indicates that a description MUST be treated as canceling the current SDP negotiation and moving the SDP offer and answer back to what it was in the previous stable state.
	RTCSdpTypeRollback
)

func (t RTCSdpType) String() string {
	switch t {
	case RTCSdpTypeOffer:
		return "offer"
	case RTCSdpTypePranswer:
		return "pranswer"
	case RTCSdpTypeAnswer:
		return "answer"
	case RTCSdpTypeRollback:
		return "rollback"
	default:
		return "Unknown"
	}
}

// RTCSessionDescription is used to expose local and remote session descriptions.
type RTCSessionDescription struct {
	Type RTCSdpType
	Sdp  string
}

// SetRemoteDescription sets the SessionDescription of the remote peer
func (r *RTCPeerConnection) SetRemoteDescription(desc RTCSessionDescription) error {
	if r.remoteDescription != nil {
		return errors.Errorf("remoteDescription is already defined, SetRemoteDescription can only be called once")
	}

	r.currentRemoteDescription = &desc

	r.remoteDescription = &sdp.SessionDescription{}

	return r.remoteDescription.Unmarshal(desc.Sdp)
}

// RTCOfferOptions describes the options used to control the offer creation process
type RTCOfferOptions struct {
	VoiceActivityDetection bool
	ICERestart             bool
}

// TODO
type candidate struct {
	transport    string
	basePriority uint16
	ip           string
	port         int
	typ          string
}

// CreateOffer starts the RTCPeerConnection and generates the localDescription
func (r *RTCPeerConnection) CreateOffer(options *RTCOfferOptions) (RTCSessionDescription, error) {
	panic("TODO")
	// if options != nil {
	// 	panic("TODO handle options")
	// }
	// if r.IsClosed {
	// 	return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	// }
	// useIdentity := r.idpLoginURL != nil
	// if useIdentity {
	// 	panic("TODO handle identity provider")
	// }

	// d := sdp.NewJSEPSessionDescription(
	// 	r.tlscfg.Fingerprint(),
	// 	useIdentity).
	// 	WithValueAttribute(sdp.AttrKeyGroup, "BUNDLE audio video") // TODO: Support BUNDLE

	// var streamlabels string
	// for _, transceiver := range r.rtpTransceivers {
	// 	if transceiver.Sender == nil ||
	// 		transceiver.Sender.Track == nil {
	// 		continue
	// 	}
	// 	track := transceiver.Sender.Track
	// 	cname := "pion"      // TODO: Support RTP streams synchronization
	// 	steamlabel := "pion" // TODO: Support steam labels
	// 	codec, err := r.mediaEngine.getCodec(track.PayloadType)
	// 	if err != nil {
	// 		return RTCSessionDescription{}, err
	// 	}
	// 	media := sdp.NewJSEPMediaDescription(track.Kind.String(), []string{}).
	// 		WithValueAttribute(sdp.AttrKeyConnectionSetup, sdp.ConnectionRoleActive.String()). // TODO: Support other connection types
	// 		WithValueAttribute(sdp.AttrKeyMID, transceiver.Mid).
	// 		WithPropertyAttribute(transceiver.Direction.String()).
	// 		WithICECredentials(r.iceAgent.Ufrag, r.iceAgent.Pwd).
	// 		WithPropertyAttribute(sdp.AttrKeyICELite).   // TODO: get ICE type from ICE Agent
	// 		WithPropertyAttribute(sdp.AttrKeyRtcpMux).   // TODO: support RTCP fallback
	// 		WithPropertyAttribute(sdp.AttrKeyRtcpRsize). // TODO: Support Reduced-Size RTCP?
	// 		WithCodec(
	// 			codec.PayloadType,
	// 			codec.Name,
	// 			codec.ClockRate,
	// 			codec.Channels,
	// 			codec.SdpFmtpLine,
	// 		).
	// 		WithMediaSource(track.Ssrc, cname, steamlabel, track.Label)
	// 	err = r.addICECandidates(media)
	// 	if err != nil {
	// 		return RTCSessionDescription{}, err
	// 	}
	// 	streamlabels = streamlabels + " " + steamlabel

	// 	d.WithMedia(media)
	// }

	// d.WithValueAttribute(sdp.AttrKeyMsidSemantic, " "+sdp.SemanticTokenWebRTCMediaStreams+streamlabels)

	// return RTCSessionDescription{
	// 	Type: RTCSdpTypeOffer,
	// 	Sdp:  d.Marshal(),
	// }, nil
}

// RTCAnswerOptions describes the options used to control the answer creation process
type RTCAnswerOptions struct {
	VoiceActivityDetection bool
}

// CreateAnswer starts the RTCPeerConnection and generates the localDescription
func (r *RTCPeerConnection) CreateAnswer(options *RTCOfferOptions) (RTCSessionDescription, error) {
	if options != nil {
		panic("TODO handle options")
	}
	if r.IsClosed {
		return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	}
	useIdentity := r.idpLoginURL != nil
	if useIdentity {
		panic("TODO handle identity provider")
	}

	candidates, err := r.buildCandidates()
	if err != nil {
		return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	}

	d := sdp.NewJSEPSessionDescription(
		r.networkManager.DTLSFingerprint(),
		useIdentity)

	bundleValue := "BUNDLE"
	for _, remoteMedia := range r.remoteDescription.MediaDescriptions {
		if strings.HasPrefix(remoteMedia.MediaName, "audio") {
			bundleValue += " audio"
			_, err := r.addAnswerMedia(d, RTCRtpCodecTypeAudio, candidates)
			if err != nil {
				return RTCSessionDescription{}, err
			}
		} else if strings.HasPrefix(remoteMedia.MediaName, "video") {
			bundleValue += " video"
			_, err := r.addAnswerMedia(d, RTCRtpCodecTypeVideo, candidates)
			if err != nil {
				return RTCSessionDescription{}, err
			}

		} else if strings.HasPrefix(remoteMedia.MediaName, "application") {
			bundleValue += " data"
			r.addAnswerData(d, candidates)
		}
	}

	d = d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue)
	return RTCSessionDescription{
		Type: RTCSdpTypeAnswer,
		Sdp:  d.Marshal(),
	}, nil
}

func (r *RTCPeerConnection) addAnswerMedia(d *sdp.SessionDescription, codecType RTCRtpCodecType, candidates []candidate) (string, error) {
	added := false

	var streamlabels string
	for _, transceiver := range r.rtpTransceivers {
		if transceiver.Sender == nil ||
			transceiver.Sender.Track == nil ||
			transceiver.Sender.Track.Kind != codecType {
			continue
		}
		track := transceiver.Sender.Track
		cname := track.Label      // TODO: Support RTP streams synchronization
		steamlabel := track.Label // TODO: Support steam labels
		codec, err := r.mediaEngine.getCodec(track.PayloadType)
		if err != nil {
			return "", err
		}
		media := sdp.NewJSEPMediaDescription(track.Kind.String(), []string{}).
			WithValueAttribute(sdp.AttrKeyConnectionSetup, sdp.ConnectionRoleActive.String()). // TODO: Support other connection types
			WithValueAttribute(sdp.AttrKeyMID, transceiver.Mid).
			WithPropertyAttribute(transceiver.Direction.String()).
			WithICECredentials(r.iceAgent.Ufrag, r.iceAgent.Pwd).
			WithPropertyAttribute(sdp.AttrKeyICELite).   // TODO: get ICE type from ICE Agent
			WithPropertyAttribute(sdp.AttrKeyRtcpMux).   // TODO: support RTCP fallback
			WithPropertyAttribute(sdp.AttrKeyRtcpRsize). // TODO: Support Reduced-Size RTCP?
			WithCodec(
				codec.PayloadType,
				codec.Name,
				codec.ClockRate,
				codec.Channels,
				codec.SdpFmtpLine,
			).
			WithMediaSource(track.Ssrc, cname, steamlabel, track.Label)

		for _, c := range candidates {
			media.WithCandidate(1, c.transport, c.basePriority, c.ip, c.port, c.typ)
		}
		media.WithPropertyAttribute("end-of-candidates") // TODO: Support full trickle-ice
		d.WithMedia(media)
		streamlabels = streamlabels + " " + steamlabel
		added = true
	}

	if !added {
		// Add media line to advertise capabilities
		media := sdp.NewJSEPMediaDescription(codecType.String(), []string{}).
			WithValueAttribute(sdp.AttrKeyConnectionSetup, sdp.ConnectionRoleActive.String()). // TODO: Support other connection types
			WithValueAttribute(sdp.AttrKeyMID, codecType.String()).
			WithPropertyAttribute(RTCRtpTransceiverDirectionSendrecv.String()).
			WithICECredentials(r.iceAgent.Ufrag, r.iceAgent.Pwd). // TODO: get credendials form ICE agent
			WithPropertyAttribute(sdp.AttrKeyICELite).            // TODO: get ICE type from ICE Agent (#23)
			WithPropertyAttribute(sdp.AttrKeyRtcpMux).            // TODO: support RTCP fallback
			WithPropertyAttribute(sdp.AttrKeyRtcpRsize)           // TODO: Support Reduced-Size RTCP?

		for _, codec := range r.mediaEngine.getCodecsByKind(codecType) {
			media.WithCodec(
				codec.PayloadType,
				codec.Name,
				codec.ClockRate,
				codec.Channels,
				codec.SdpFmtpLine,
			)
		}

		for _, c := range candidates {
			media.WithCandidate(1, c.transport, c.basePriority, c.ip, c.port, c.typ)
		}
		media.WithPropertyAttribute("end-of-candidates") // TODO: Support full trickle-ice
		d.WithMedia(media)
	}

	return streamlabels, nil

}

func (r *RTCPeerConnection) addAnswerData(d *sdp.SessionDescription, candidates []candidate) {
	media := (&sdp.MediaDescription{
		MediaName:      "application 9 DTLS/SCTP 5000",
		ConnectionData: "IN IP4 0.0.0.0",
		Attributes:     []string{},
	}).
		WithValueAttribute(sdp.AttrKeyConnectionSetup, sdp.ConnectionRoleActive.String()). // TODO: Support other connection types
		WithValueAttribute(sdp.AttrKeyMID, "data").
		WithValueAttribute("sctpmap:5000", "webrtc-datachannel 1024").
		WithICECredentials(r.iceAgent.Ufrag, r.iceAgent.Pwd).
		WithPropertyAttribute(sdp.AttrKeyICELite) // TODO: get ICE type from ICE Agent

	for _, c := range candidates {
		media.WithCandidate(1, c.transport, c.basePriority, c.ip, c.port, c.typ)
	}
	media.WithPropertyAttribute("end-of-candidates") // TODO: Support full trickle-ice

	d.WithMedia(media)
}

func (r *RTCPeerConnection) buildCandidates() ([]candidate, error) {
	basePriority := uint16(rand.Uint32() & (1<<16 - 1))
	candidates := make([]candidate, 0)

	for _, c := range ice.HostInterfaces() {
		boundAddress, err := r.networkManager.Listen(c + ":0")
		if err != nil {
			return nil, err
		}

		candidates = append(candidates, candidate{
			transport:    "udp",
			basePriority: basePriority,
			ip:           boundAddress.IP.String(),
			port:         boundAddress.Port,
			typ:          "host",
		})

		basePriority = basePriority + 1
	}

	for _, servers := range r.iceAgent.Servers {
		for _, server := range servers {
			if server.Type != ice.ServerTypeSTUN {
				continue
			}
			// TODO Do we want the timeout to be configurable?
			proto := server.TransportType.String()
			client, err := stun.NewClient(proto, fmt.Sprintf("%s:%d", server.Host, server.Port), time.Second*5)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to create STUN client")
			}
			localAddr, ok := client.LocalAddr().(*net.UDPAddr)
			if !ok {
				return nil, errors.Errorf("Failed to cast STUN client to UDPAddr")
			}

			resp, err := client.Request()
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to make STUN request")
			}

			if err = client.Close(); err != nil {
				return nil, errors.Wrapf(err, "Failed to close STUN client")
			}

			attr, ok := resp.GetOneAttribute(stun.AttrXORMappedAddress)
			if !ok {
				return nil, errors.Errorf("Got respond from STUN server that did not contain XORAddress")
			}

			var addr stun.XorAddress
			if err = addr.Unpack(resp, attr); err != nil {
				return nil, errors.Wrapf(err, "Failed to unpack STUN XorAddress response")
			}

			boundAddress, err := r.networkManager.Listen(fmt.Sprintf("0.0.0.0:%d", localAddr.Port))
			if err != nil {
				return nil, err
			}

			candidates = append(candidates, candidate{
				transport:    "udp",
				basePriority: basePriority,
				ip:           addr.IP.String(),
				port:         boundAddress.Port,
				typ:          "srflx",
			})

			basePriority = basePriority + 1
		}
	}

	return candidates, nil
}
