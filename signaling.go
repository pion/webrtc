package webrtc

import (
	"fmt"
	"strings"

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

	weOffer := true
	remoteUfrag := ""
	remotePwd := ""
	if desc.Type == RTCSdpTypeOffer {
		weOffer = false
	}

	r.currentRemoteDescription = &desc
	r.remoteDescription = &sdp.SessionDescription{}
	if err := r.remoteDescription.Unmarshal(desc.Sdp); err != nil {
		return err
	}

	for _, m := range r.remoteDescription.MediaDescriptions {
		for _, a := range m.Attributes {
			if strings.HasPrefix(a, "candidate") {
				if c := sdp.ICECandidateUnmarshal(a); c != nil {
					r.networkManager.IceAgent.AddRemoteCandidate(c)
				} else {
					fmt.Printf("Tried to parse ICE candidate, but failed %s ", a)
				}
			} else if strings.HasPrefix(a, "ice-ufrag") {
				remoteUfrag = a[len("ice-ufrag")+1:]
			} else if strings.HasPrefix(a, "ice-pwd") {
				remotePwd = a[len("ice-pwd")+1:]
			}
		}
	}

	r.networkManager.Start(weOffer, remoteUfrag, remotePwd)
	return nil
}

func (r *RTCPeerConnection) generateLocalCandidates() []string {
	r.networkManager.IceAgent.RLock()
	defer r.networkManager.IceAgent.RUnlock()

	candidates := make([]string, 0)
	for _, c := range r.networkManager.IceAgent.LocalCandidates {
		candidates = append(candidates, sdp.ICECandidateMarshal(c)...)
	}
	return candidates
}

// CreateOffer starts the RTCPeerConnection and generates the localDescription
func (r *RTCPeerConnection) CreateOffer(options *RTCOfferOptions) (RTCSessionDescription, error) {
	useIdentity := r.idpLoginURL != nil
	if options != nil {
		panic("TODO handle options")
	} else if useIdentity {
		panic("TODO handle identity provider")
	} else if r.IsClosed {
		return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	}

	d := sdp.NewJSEPSessionDescription(r.networkManager.DTLSFingerprint(), useIdentity)
	candidates := r.generateLocalCandidates()

	r.addRTPMediaSections(d, []RTCRtpCodecType{RTCRtpCodecTypeAudio, RTCRtpCodecTypeVideo}, candidates)
	r.addDataMediaSection(d, candidates)
	d = d.WithValueAttribute(sdp.AttrKeyGroup, "BUNDLE audio video data")

	for _, m := range d.MediaDescriptions {
		m.WithPropertyAttribute("setup:actpass")
	}

	return RTCSessionDescription{
		Type: RTCSdpTypeOffer,
		Sdp:  d.Marshal(),
	}, nil
}

// CreateAnswer starts the RTCPeerConnection and generates the localDescription
func (r *RTCPeerConnection) CreateAnswer(options *RTCAnswerOptions) (RTCSessionDescription, error) {
	useIdentity := r.idpLoginURL != nil
	if options != nil {
		panic("TODO handle options")
	} else if useIdentity {
		panic("TODO handle identity provider")
	} else if r.IsClosed {
		return RTCSessionDescription{}, &InvalidStateError{Err: ErrConnectionClosed}
	}

	bundleValue := "BUNDLE"
	mediaSectionsToAdd := []RTCRtpCodecType{}
	addData := false
	for _, remoteMedia := range r.remoteDescription.MediaDescriptions {
		if strings.HasPrefix(remoteMedia.MediaName, "audio") {
			bundleValue += " audio"
			mediaSectionsToAdd = append(mediaSectionsToAdd, RTCRtpCodecTypeAudio)
		} else if strings.HasPrefix(remoteMedia.MediaName, "video") {
			bundleValue += " video"
			mediaSectionsToAdd = append(mediaSectionsToAdd, RTCRtpCodecTypeVideo)
		} else if strings.HasPrefix(remoteMedia.MediaName, "application") {
			bundleValue += " data"
			addData = true
		}
	}

	candidates := r.generateLocalCandidates()
	d := sdp.NewJSEPSessionDescription(r.networkManager.DTLSFingerprint(), useIdentity)

	r.addRTPMediaSections(d, mediaSectionsToAdd, candidates)
	if addData {
		r.addDataMediaSection(d, candidates)
	}
	d = d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue)

	return RTCSessionDescription{
		Type: RTCSdpTypeAnswer,
		Sdp:  d.Marshal(),
	}, nil
}

/*
	TODO If we are generating an offer only include media sections we want
*/
func (r *RTCPeerConnection) addRTPMediaSections(d *sdp.SessionDescription, mediaTypes []RTCRtpCodecType, candidates []string) {
	addMediaType := func(codecType RTCRtpCodecType) {
		media := sdp.NewJSEPMediaDescription(codecType.String(), []string{}).
			WithValueAttribute(sdp.AttrKeyConnectionSetup, sdp.ConnectionRoleActive.String()). // TODO: Support other connection types
			WithValueAttribute(sdp.AttrKeyMID, codecType.String()).
			WithPropertyAttribute(RTCRtpTransceiverDirectionSendrecv.String()).
			WithICECredentials(r.networkManager.IceAgent.LocalUfrag, r.networkManager.IceAgent.LocalPwd).
			WithPropertyAttribute(sdp.AttrKeyRtcpMux).  // TODO: support RTCP fallback
			WithPropertyAttribute(sdp.AttrKeyRtcpRsize) // TODO: Support Reduced-Size RTCP?

		for _, codec := range r.mediaEngine.getCodecsByKind(codecType) {
			media.WithCodec(codec.PayloadType, codec.Name, codec.ClockRate, codec.Channels, codec.SdpFmtpLine)
		}

		for _, transceiver := range r.rtpTransceivers {
			if transceiver.Sender == nil ||
				transceiver.Sender.Track == nil ||
				transceiver.Sender.Track.Kind != codecType {
				continue
			}
			track := transceiver.Sender.Track
			media = media.WithMediaSource(track.Ssrc, track.Label /* cname */, track.Label /* streamLabel */, track.Label)
		}

		for _, c := range candidates {
			media.WithCandidate(c)
		}
		media.WithPropertyAttribute("end-of-candidates")
		d.WithMedia(media)

	}

	for _, c := range mediaTypes {
		addMediaType(c)
	}
}

func (r *RTCPeerConnection) addDataMediaSection(d *sdp.SessionDescription, candidates []string) {
	media := (&sdp.MediaDescription{
		MediaName:      "application 9 DTLS/SCTP 5000",
		ConnectionData: "IN IP4 0.0.0.0",
		Attributes:     []string{},
	}).
		WithValueAttribute(sdp.AttrKeyConnectionSetup, sdp.ConnectionRoleActive.String()). // TODO: Support other connection types
		WithValueAttribute(sdp.AttrKeyMID, "data").
		WithValueAttribute("sctpmap:5000", "webrtc-datachannel 1024").
		WithICECredentials(r.networkManager.IceAgent.LocalUfrag, r.networkManager.IceAgent.LocalPwd)

	for _, c := range candidates {
		media.WithCandidate(c)
	}
	media.WithPropertyAttribute("end-of-candidates")

	d.WithMedia(media)
}
