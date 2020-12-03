// +build !js

package webrtc

import (
	"sync/atomic"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
)

type interceptorTrackLocalWriter struct {
	TrackLocalWriter
	rtpWriter atomic.Value
}

func (i *interceptorTrackLocalWriter) setRTPWriter(writer interceptor.RTPWriter) {
	i.rtpWriter.Store(writer)
}

func (i *interceptorTrackLocalWriter) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	writer := i.rtpWriter.Load().(interceptor.RTPWriter)

	if writer == nil {
		return 0, nil
	}

	return writer.Write(&rtp.Packet{Header: *header, Payload: payload}, make(interceptor.Attributes))
}
