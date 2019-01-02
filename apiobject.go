package webrtc

// API is a repository for semi-global settings to WebRTC objects
// In the simplest case, the DefaultAPI object should be used
// rather then constructing a new API object.
type API struct {
	settingEngine settingEngine
	mediaEngine   MediaEngine
}

var defaultAPI = NewAPI()

// NewAPI Creates a new API object for keeping semi-global settings to WebRTC objects
func NewAPI() *API {
	a := new(API)
	initSettingEngine(&a.settingEngine)
	InitMediaEngine(&a.mediaEngine)
	return a
}
