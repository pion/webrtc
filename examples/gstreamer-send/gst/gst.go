package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline   *C.GstElement
	in         chan<- []byte
}

// This allows cgo to access pipeline, this will not work if you want multiple
var globalPipeline *Pipeline

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(in chan<- []byte) *Pipeline {
	globalPipeline = &Pipeline{
		Pipeline:   C.gstreamer_send_create_pipeline(),
		in:         in,
	}

	return globalPipeline
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_send_start_pipeline(p.Pipeline)
}

// Stop stops the GStreamer Pipeline
func (p *Pipeline) Stop() {
	C.gstreamer_send_stop_pipeline(p.Pipeline)
}

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int) {
	if globalPipeline != nil {
		globalPipeline.in <- C.GoBytes(buffer, bufferLen)
	} else {
		fmt.Println("discarding buffer, globalPipeline not set")
	}
	C.free(buffer)
}
