// +build !js

package webrtc

import (
	"github.com/pion/logging"
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
func RegisterDefaultInterceptors(settingEngine *SettingEngine, mediaEngine *MediaEngine, interceptorRegistry *InterceptorRegistry) error {
	loggerFactory := settingEngine.LoggerFactory
	if loggerFactory == nil {
		loggerFactory = logging.NewDefaultLoggerFactory()
	}

	err := ConfigureNack(loggerFactory, mediaEngine, interceptorRegistry)
	if err != nil {
		return err
	}

	return nil
}

// ConfigureNack will setup everything necessary for handling generating/responding to nack messages.
func ConfigureNack(loggerFactory logging.LoggerFactory, mediaEngine *MediaEngine, interceptorRegistry *InterceptorRegistry) error {
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: "nack"}, RTPCodecTypeVideo)
	receiverNack, err := interceptor.NewReceiverNack(8192, loggerFactory.NewLogger("receiver_nack"))
	if err != nil {
		return err
	}
	interceptorRegistry.Add(receiverNack)
	senderNack, err := interceptor.NewSenderNack(8192, loggerFactory.NewLogger("sender_nack"))
	if err != nil {
		return err
	}
	interceptorRegistry.Add(senderNack)

	return nil
}
