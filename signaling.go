package webrtc

import (
	"fmt"
	"strings"

	"net"

	"github.com/pions/webrtc/internal/sdp"
	"github.com/pkg/errors"
)

// RTCAnswerOptions describes the options used to control the answer creation process
type RTCAnswerOptions struct {
	VoiceActivityDetection bool
}

// RTCOfferOptions describes the options used to control the offer creation process
type RTCOfferOptions struct {
	VoiceActivityDetection bool
	ICERestart             bool
}

// RTCSignalingState indicates the state of the offer/answer process
type RTCSignalingState int

const (
	// RTCSignalingStateStable indicates there is no offer­answer exchange in progress.
	RTCSignalingStateStable RTCSignalingState = iota + 1

	// RTCSignalingStateHaveLocalOffer indicates A local description, of type "offer", has been successfully applied.
	RTCSignalingStateHaveLocalOffer

	// RTCSignalingStateHaveRemoteOffer indicates A remote description, of type "offer", has been successfully applied.
	RTCSignalingStateHaveRemoteOffer

	// RTCSignalingStateHaveLocalPranswer indicates A remote description of type "offer" has been successfully applied
	// and a local description of type "pranswer" has been successfully applied.
	RTCSignalingStateHaveLocalPranswer

	// RTCSignalingStateHaveRemotePranswer indicates A local description of type "offer" has been successfully applied
	// and a remote description of type "pranswer" has been successfully applied.
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
		return ErrUnknownType.Error()
	}
}

// RTCSdpType describes the type of an RTCSessionDescription
type RTCSdpType int

const (
	// RTCSdpTypeOffer indicates that a description MUST be treated as an SDP offer.
	RTCSdpTypeOffer RTCSdpType = iota + 1

	// RTCSdpTypePranswer indicates that a description MUST be treated as an SDP answer, but not a final answer.
	RTCSdpTypePranswer

	// RTCSdpTypeAnswer indicates that a description MUST be treated as an SDP final answer, and the offer-answer
	// exchange MUST be considered complete.
	RTCSdpTypeAnswer

	// RTCSdpTypeRollback indicates that a description MUST be treated as canceling the current SDP negotiation
	// and moving the SDP offer and answer back to what it was in the previous stable state.
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
		return ErrUnknownType.Error()
	}
}

// RTCSessionDescription is used to expose local and remote session descriptions.
type RTCSessionDescription struct {
	Type RTCSdpType
	Sdp  string

	// This will never be initialized by callers, internal use only
	parsed *sdp.SessionDescription
}

// SetRemoteDescription sets the SessionDescription of the remote peer
func (pc *RTCPeerConnection) SetRemoteDescription(desc RTCSessionDescription) error {
	if pc.CurrentRemoteDescription != nil {
		return errors.Errorf("remoteDescription is already defined, SetRemoteDescription can only be called once")
	}

	weOffer := true
	remoteUfrag := ""
	remotePwd := ""
	if desc.Type == RTCSdpTypeOffer {
		weOffer = false
	}

	pc.CurrentRemoteDescription = &desc
	pc.CurrentRemoteDescription.parsed = &sdp.SessionDescription{}
	if err := pc.CurrentRemoteDescription.parsed.Unmarshal(pc.CurrentRemoteDescription.Sdp); err != nil {
		return err
	}

	for _, m := range pc.CurrentRemoteDescription.parsed.MediaDescriptions {
		for _, a := range m.Attributes {
			if strings.HasPrefix(*a.String(), "candidate") {
				if c := sdp.ICECandidateUnmarshal(*a.String()); c != nil {
					// pc.networkManager.IceAgent.AddRemoteCandidate(c)
				} else {
					fmt.Printf("Tried to parse ICE candidate, but failed %s ", a)
				}
			} else if strings.HasPrefix(*a.String(), "ice-ufrag") {
				remoteUfrag = (*a.String())[len("ice-ufrag:"):]
			} else if strings.HasPrefix(*a.String(), "ice-pwd") {
				remotePwd = (*a.String())[len("ice-pwd:"):]
			}
		}
	}
	return pc.networkManager.Start(weOffer, remoteUfrag, remotePwd)
}

func (pc *RTCPeerConnection) generateLocalCandidates() []string {
	// pc.networkManager.IceAgent.RLock()
	// defer pc.networkManager.IceAgent.RUnlock()

	candidates := make([]string, 0)
	// for _, c := range pc.networkManager.IceAgent.LocalCandidates {
	// 	candidates = append(candidates, sdp.ICECandidateMarshal(c)...)
	// }
	return candidates
}

// CreateOffer starts the RTCPeerConnection and generates the localDescription
func (pc *RTCPeerConnection) CreateOffer(options *RTCOfferOptions) (RTCSessionDescription, error) {
	useIdentity := pc.idpLoginURL != nil
	if options != nil {
		return RTCSessionDescription{}, errors.Errorf("TODO handle options")
	} else if useIdentity {
		return RTCSessionDescription{}, errors.Errorf("TODO handle identity provider")
	} else if pc.IsClosed {
		return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	}

	d := sdp.NewJSEPSessionDescription(pc.networkManager.DTLSFingerprint(), useIdentity)
	candidates := pc.generateLocalCandidates()

	bundleValue := "BUNDLE"

	if pc.addRTPMediaSection(d, RTCRtpCodecTypeAudio, "audio", RTCRtpTransceiverDirectionSendrecv, candidates, sdp.ConnectionRoleActpass) {
		bundleValue += " audio"
	}
	if pc.addRTPMediaSection(d, RTCRtpCodecTypeVideo, "video", RTCRtpTransceiverDirectionSendrecv, candidates, sdp.ConnectionRoleActpass) {
		bundleValue += " video"
	}

	pc.addDataMediaSection(d, "data", candidates, sdp.ConnectionRoleActpass)
	d = d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue+" data")

	for _, m := range d.MediaDescriptions {
		m.WithPropertyAttribute("setup:actpass")
	}

	pc.CurrentLocalDescription = &RTCSessionDescription{
		Type:   RTCSdpTypeOffer,
		Sdp:    d.Marshal(),
		parsed: d,
	}

	return *pc.CurrentLocalDescription, nil
}

// CreateAnswer starts the RTCPeerConnection and generates the localDescription
func (pc *RTCPeerConnection) CreateAnswer(options *RTCAnswerOptions) (RTCSessionDescription, error) {
	useIdentity := pc.idpLoginURL != nil
	if options != nil {
		return RTCSessionDescription{}, errors.Errorf("TODO handle options")
	} else if useIdentity {
		return RTCSessionDescription{}, errors.Errorf("TODO handle identity provider")
	} else if pc.IsClosed {
		return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	}

	candidates := pc.generateLocalCandidates()
	d := sdp.NewJSEPSessionDescription(pc.networkManager.DTLSFingerprint(), useIdentity)

	bundleValue := "BUNDLE"
	for _, remoteMedia := range pc.CurrentRemoteDescription.parsed.MediaDescriptions {
		// TODO @trivigy better SDP parser
		var peerDirection RTCRtpTransceiverDirection
		midValue := ""
		for _, a := range remoteMedia.Attributes {
			if strings.HasPrefix(*a.String(), "mid") {
				midValue = (*a.String())[len("mid:"):]
			} else if strings.HasPrefix(*a.String(), "sendrecv") {
				peerDirection = RTCRtpTransceiverDirectionSendrecv
			} else if strings.HasPrefix(*a.String(), "sendonly") {
				peerDirection = RTCRtpTransceiverDirectionSendonly
			} else if strings.HasPrefix(*a.String(), "recvonly") {
				peerDirection = RTCRtpTransceiverDirectionRecvonly
			}
		}

		appendBundle := func() {
			bundleValue += " " + midValue
		}

		if strings.HasPrefix(*remoteMedia.MediaName.String(), "audio") {
			if pc.addRTPMediaSection(d, RTCRtpCodecTypeAudio, midValue, peerDirection, candidates, sdp.ConnectionRoleActive) {
				appendBundle()
			}
		} else if strings.HasPrefix(*remoteMedia.MediaName.String(), "video") {
			if pc.addRTPMediaSection(d, RTCRtpCodecTypeVideo, midValue, peerDirection, candidates, sdp.ConnectionRoleActive) {
				appendBundle()
			}
		} else if strings.HasPrefix(*remoteMedia.MediaName.String(), "application") {
			pc.addDataMediaSection(d, midValue, candidates, sdp.ConnectionRoleActive)
		}
	}

	d = d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue)

	pc.CurrentLocalDescription = &RTCSessionDescription{
		Type:   RTCSdpTypeAnswer,
		Sdp:    d.Marshal(),
		parsed: d,
	}
	return *pc.CurrentLocalDescription, nil
}

func localDirection(weSend bool, peerDirection RTCRtpTransceiverDirection) RTCRtpTransceiverDirection {
	theySend := (peerDirection == RTCRtpTransceiverDirectionSendrecv || peerDirection == RTCRtpTransceiverDirectionSendonly)
	if weSend && theySend {
		return RTCRtpTransceiverDirectionSendrecv
	} else if weSend && !theySend {
		return RTCRtpTransceiverDirectionSendonly
	} else if !weSend && theySend {
		return RTCRtpTransceiverDirectionRecvonly
	}

	return RTCRtpTransceiverDirectionInactive
}

func (pc *RTCPeerConnection) addRTPMediaSection(d *sdp.SessionDescription, codecType RTCRtpCodecType, midValue string, peerDirection RTCRtpTransceiverDirection, candidates []string, dtlsRole sdp.ConnectionRole) bool {
	if codecs := pc.mediaEngine.getCodecsByKind(codecType); len(codecs) == 0 {
		return false
	}

	media := sdp.NewJSEPMediaDescription(codecType.String(), []string{})
	// WithValueAttribute(sdp.AttrKeyConnectionSetup, dtlsRole.String()). // TODO: Support other connection types
	// WithValueAttribute(sdp.AttrKeyMID, midValue).
	// WithICECredentials(pc.networkManager.IceAgent.LocalUfrag, pc.networkManager.IceAgent.LocalPwd).
	// WithPropertyAttribute(sdp.AttrKeyRtcpMux).  // TODO: support RTCP fallback
	// WithPropertyAttribute(sdp.AttrKeyRtcpRsize) // TODO: Support Reduced-Size RTCP?

	for _, codec := range pc.mediaEngine.getCodecsByKind(codecType) {
		media.WithCodec(codec.PayloadType, codec.Name, codec.ClockRate, codec.Channels, codec.SdpFmtpLine)
	}

	weSend := false
	for _, transceiver := range pc.rtpTransceivers {
		if transceiver.Sender == nil ||
			transceiver.Sender.Track == nil ||
			transceiver.Sender.Track.Kind != codecType {
			continue
		}
		weSend = true
		track := transceiver.Sender.Track
		media = media.WithMediaSource(track.Ssrc, track.Label /* cname */, track.Label /* streamLabel */, track.Label)
	}
	media = media.WithPropertyAttribute(localDirection(weSend, peerDirection).String())

	for _, c := range candidates {
		media.WithCandidate(c)
	}
	media.WithPropertyAttribute("end-of-candidates")
	d.WithMedia(media)
	return true
}

func (pc *RTCPeerConnection) addDataMediaSection(d *sdp.SessionDescription, midValue string, candidates []string, dtlsRole sdp.ConnectionRole) {
	media := &sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media:   "application",
			Port:    sdp.RangedPort{Value: 9},
			Protos:  []string{"DTLS", "SCTP"},
			Formats: []int{5000},
		},
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address: &sdp.Address{
				IP: net.ParseIP("0.0.0.0"),
			},
		},
	}
	// WithValueAttribute(sdp.AttrKeyConnectionSetup, dtlsRole.String()). // TODO: Support other connection types
	// WithValueAttribute(sdp.AttrKeyMID, midValue).
	// WithPropertyAttribute(RTCRtpTransceiverDirectionSendrecv.String()).
	// WithPropertyAttribute("sctpmap:5000 webrtc-datachannel 1024").
	// WithICECredentials(pc.networkManager.IceAgent.LocalUfrag, pc.networkManager.IceAgent.LocalPwd)

	for _, c := range candidates {
		media.WithCandidate(c)
	}
	media.WithPropertyAttribute("end-of-candidates")

	d.WithMedia(media)
}
