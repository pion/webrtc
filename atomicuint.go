package webrtc

import (
	"sync/atomic"
)

type atomicUint32 struct {
	av *atomic.Value
}

func NewAtomicUint32() atomicUint32 {
	value := &atomic.Value{}
	value.Store(uint32(0))
	return atomicUint32{av: value}
}

func (a atomicUint32) increment() {
	if a.av == nil {
		a.av = &atomic.Value{}
	}

	a.av.Store(a.value() + uint32(1))
}

func (a atomicUint32) value() uint32 {
	if a.av == nil {
		a.av = &atomic.Value{}
		return 0
	}

	value, ok := a.av.Load().(uint32)
	if ok {
		return value
	}

	return 0
}

func (a atomicUint32) add(quantity uint32) {
	if a.av == nil {
		a.av = &atomic.Value{}
	}

	a.av.Store(a.value() + quantity)
}
