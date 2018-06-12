package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"unsafe"
)

type Pipeline struct {
	Pipeline *C.GstElement
}

func CreatePipeline() *Pipeline {
	p := &Pipeline{}
	p.Pipeline = C.gst_create_pipeline()
	return p
}

func (p *Pipeline) Start() {
	C.gst_start_pipeline(p.Pipeline)
}

func (p *Pipeline) Stop() {
	C.gst_stop_pipeline(p.Pipeline)
}

func (p *Pipeline) Push(buffer []byte) {
	b := C.CBytes(buffer)
	defer C.free(unsafe.Pointer(b))
	C.gst_push_buffer(p.Pipeline, b, C.int(len(buffer)))
}
