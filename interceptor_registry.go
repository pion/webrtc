// +build !js

package webrtc

import (
	"github.com/pion/webrtc/v3/pkg/interceptor"
)

// InterceptorRegistry is a collector for interceptors.
type InterceptorRegistry struct {
	interceptors []interceptor.Interceptor
}

// Add adds a new Interceptor to the registry.
func (i *InterceptorRegistry) Add(icpr interceptor.Interceptor) {
	i.interceptors = append(i.interceptors, icpr)
}

func (i *InterceptorRegistry) build() interceptor.Interceptor {
	if len(i.interceptors) == 0 {
		return &interceptor.NoOp{}
	}

	return interceptor.NewChain(i.interceptors)
}

// RegisterDefaultInterceptors will register some useful interceptors. If you want to customize which interceptors are loaded,
// you should copy the code from this method and remove unwanted interceptors.
func RegisterDefaultInterceptors(mediaEngine *MediaEngine, interceptorRegistry *InterceptorRegistry) error {
	err := ConfigureNack(mediaEngine, interceptorRegistry)
	if err != nil {
		return err
	}

	return nil
}

// ConfigureNack will setup everything necessary for handling generating/responding to nack messages.
func ConfigureNack(mediaEngine *MediaEngine, interceptorRegistry *InterceptorRegistry) error {
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: "nack"}, RTPCodecTypeVideo)
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: "nack", Parameter: "pli"}, RTPCodecTypeVideo)
	interceptorRegistry.Add(&interceptor.NACK{})
	return nil
}
