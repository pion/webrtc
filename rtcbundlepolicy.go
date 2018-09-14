package webrtc

// RTCBundlePolicy affects which media tracks are negotiated if the remote
// endpoint is not bundle-aware, and what ICE candidates are gathered. If the
// remote endpoint is bundle-aware, all media tracks and data channels are
// bundled onto the same transport.
type RTCBundlePolicy int

const (
	// RTCBundlePolicyBalanced indicates to gather ICE candidates for each
	// media type in use (audio, video, and data). If the remote endpoint is
	// not bundle-aware, negotiate only one audio and video track on separate
	// transports.
	RTCBundlePolicyBalanced RTCBundlePolicy = iota + 1

	// RTCBundlePolicyMaxCompat indicates to gather ICE candidates for each
	// track. If the remote endpoint is not bundle-aware, negotiate all media
	// tracks on separate transports.
	RTCBundlePolicyMaxCompat

	// RTCBundlePolicyMaxBundle indicates to gather ICE candidates for only
	// one track. If the remote endpoint is not bundle-aware, negotiate only
	// one media track.
	RTCBundlePolicyMaxBundle
)

// This is done this way because of a linter.
const (
	rtcBundlePolicyBalancedStr  = "balanced"
	rtcBundlePolicyMaxCompatStr = "max-compat"
	rtcBundlePolicyMaxBundleStr = "max-bundle"
)

func newRTCBundlePolicy(raw string) RTCBundlePolicy {
	switch raw {
	case rtcBundlePolicyBalancedStr:
		return RTCBundlePolicyBalanced
	case rtcBundlePolicyMaxCompatStr:
		return RTCBundlePolicyMaxCompat
	case rtcBundlePolicyMaxBundleStr:
		return RTCBundlePolicyMaxBundle
	default:
		return RTCBundlePolicy(Unknown)
	}
}

func (t RTCBundlePolicy) String() string {
	switch t {
	case RTCBundlePolicyBalanced:
		return rtcBundlePolicyBalancedStr
	case RTCBundlePolicyMaxCompat:
		return rtcBundlePolicyMaxCompatStr
	case RTCBundlePolicyMaxBundle:
		return rtcBundlePolicyMaxBundleStr
	default:
		return ErrUnknownType.Error()
	}
}
