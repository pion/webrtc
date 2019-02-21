package webrtc

// API bundles the global funcions of the WebRTC and ORTC API.
// Some of these functions are also exported globally using the
// defaultAPI object. Note that the global version of the API
// may be phased out in the future.
type API struct {
	settingEngine *SettingEngine
	mediaEngine   *MediaEngine
}

// NewAPI Creates a new API object for keeping semi-global settings to WebRTC objects
func NewAPI(options ...func(*API)) *API {
	a := &API{}

	for _, o := range options {
		o(a)
	}

	if a.settingEngine == nil {
		a.settingEngine = &SettingEngine{}
	}

	if a.mediaEngine == nil {
		a.mediaEngine = &MediaEngine{}
	}

	return a
}

// WithMediaEngine allows providing a MediaEngine to the API.
// Settings should not be changed after passing the engine to an API.
func WithMediaEngine(m MediaEngine) func(a *API) {
	return func(a *API) {
		a.mediaEngine = &m
	}
}

// WithSettingEngine allows providing a SettingEngine to the API.
// Settings should not be changed after passing the engine to an API.
func WithSettingEngine(s SettingEngine) func(a *API) {
	return func(a *API) {
		a.settingEngine = &s
	}
}

// defaultAPI is used to support the legacy global API.
// This global API should not be extended and may be phased out
// in the future.
var defaultAPI = NewAPI()

// Media Engine API

// RegisterCodec on the default API.
// See MediaEngine for details.
func RegisterCodec(codec *RTPCodec) {
	defaultAPI.mediaEngine.RegisterCodec(codec)
}

// RegisterDefaultCodecs on the default API.
// See MediaEngine for details.
func RegisterDefaultCodecs() {
	defaultAPI.mediaEngine.RegisterDefaultCodecs()
}

// PeerConnection API

// NewPeerConnection using the default API.
// See API.NewRTCPeerConnection for details.
func NewPeerConnection(configuration Configuration) (*PeerConnection, error) {
	return defaultAPI.NewPeerConnection(configuration)
}
