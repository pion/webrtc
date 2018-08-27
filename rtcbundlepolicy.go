package webrtc

// RTCBundlePolicy affects which media tracks are negotiated if the remote endpoint is not bundle-aware,
// and what ICE candidates are gathered.
type RTCBundlePolicy int

const (

	// RTCBundlePolicyBalanced indicates to gather ICE candidates for each media type in use (audio, video, and data).
	RTCBundlePolicyBalanced RTCBundlePolicy = iota + 1

	// RTCBundlePolicyMaxCompat indicates to gather ICE candidates for each track.
	RTCBundlePolicyMaxCompat

	// RTCBundlePolicyMaxBundle indicates to gather ICE candidates for only one track.
	RTCBundlePolicyMaxBundle
)

func NewRTCBundlePolicy(raw string) (unknown RTCBundlePolicy) {
	switch raw {
	case "balanced":
		return RTCBundlePolicyBalanced
	case "max-compat":
		return RTCBundlePolicyMaxCompat
	case "max-bundle":
		return RTCBundlePolicyMaxBundle
	default:
		return unknown
	}
}

func (t RTCBundlePolicy) String() string {
	switch t {
	case RTCBundlePolicyBalanced:
		return "balanced"
	case RTCBundlePolicyMaxCompat:
		return "max-compat"
	case RTCBundlePolicyMaxBundle:
		return "max-bundle"
	default:
		return ErrUnknownType.Error()
	}
}
