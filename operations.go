package webrtc

import (
	"sync"
)

// Operation is a function
type operation func()

// Operations is a task executor.
type operations struct {
	ops     []operation
	mu      sync.Mutex
	startMu sync.Mutex
}

func newOperations() *operations {
	return &operations{}
}

// Enqueue adds a new action to be executed. If there are no actions scheduled,
// the execution will start immediately in a new goroutine.
func (o *operations) Enqueue(op operation) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ops = append(o.ops, op)
	if len(o.ops) == 1 {
		go func() {
			o.startMu.Lock()
			defer o.startMu.Unlock()
			o.start()
		}()
	}
}

// Done blocks until all currently enqueued operations are finished executing.
func (o *operations) Done() {
	var wg sync.WaitGroup
	wg.Add(1)
	o.Enqueue(func() {
		wg.Done()
	})
	wg.Wait()
}

func (o *operations) pop() (fn func(), isLast bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fn = o.ops[0]
	o.ops = o.ops[1:]
	return fn, len(o.ops) == 0
}

func (o *operations) start() {
	var fn func()
	isLast := false
	for !isLast {
		fn, isLast = o.pop()
		fn()
	}
}
