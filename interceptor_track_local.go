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
	if writer, ok := i.rtpWriter.Load().(interceptor.RTPWriter); ok && writer != nil {
		return writer.Write(&rtp.Packet{Header: *header, Payload: payload}, make(interceptor.Attributes))
	}

	return 0, nil
}
