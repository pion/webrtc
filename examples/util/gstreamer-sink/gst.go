package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"unsafe"

	"github.com/pions/webrtc"
)

func init() {
	go C.gstreamer_recieve_start_mainloop()
}

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline *C.GstElement
}

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(codecName string) *Pipeline {
	pipelineStr := "appsrc format=time is-live=true do-timestamp=true name=src ! application/x-rtp"
	switch codecName {
	case webrtc.VP8:
		pipelineStr += ", encoding-name=VP8-DRAFT-IETF-01 ! rtpvp8depay ! decodebin ! autovideosink"
	case webrtc.Opus:
		pipelineStr += ", payload=96, encoding-name=OPUS ! rtpopusdepay ! decodebin ! autoaudiosink"
	case webrtc.VP9:
		pipelineStr += " ! rtpvp9depay ! decodebin ! autovideosink"
	case webrtc.H264:
		pipelineStr += " ! rtph264depay ! decodebin ! autovideosink"
	default:
		panic("Unhandled codec " + codecName)
	}

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))
	return &Pipeline{Pipeline: C.gstreamer_recieve_create_pipeline(pipelineStrUnsafe)}
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_recieve_start_pipeline(p.Pipeline)
}

// Stop stops the GStreamer Pipeline
func (p *Pipeline) Stop() {
	C.gstreamer_recieve_stop_pipeline(p.Pipeline)
}

// Push pushes a buffer on the appsrc of the GStreamer Pipeline
func (p *Pipeline) Push(buffer []byte) {
	b := C.CBytes(buffer)
	defer C.free(unsafe.Pointer(b))
	C.gstreamer_recieve_push_buffer(p.Pipeline, b, C.int(len(buffer)))
}
