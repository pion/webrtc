package webrtc

import (
	"sync"
)

// Operation is a function
type operation func()

// Operations is a task executor.
type operations struct {
	mu          sync.Mutex
	busy        bool
	busyHandler func()
	ops         []operation
}

func newOperations() *operations {
	return &operations{}
}

// Enqueue adds a new action to be executed. If there are no actions scheduled,
// the execution will start immediately in a new goroutine.
func (o *operations) Enqueue(op operation) {
	if op == nil {
		return
	}

	o.mu.Lock()
	running := o.busy
	o.ops = append(o.ops, op)
	o.busy = true
	o.mu.Unlock()

	if !running {
		go o.start()
	}
}

// IsEmpty checks if the queue is empty
func (o *operations) IsEmpty() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.ops) == 0
}

// OnBusy called when the queue is busy
func (o *operations) OnBusy(f func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.busyHandler = f
}

// Done blocks until all currently enqueued operations are finished executing.
// For more complex synchronization, use Enqueue directly.
func (o *operations) Done() {
	var wg sync.WaitGroup
	wg.Add(1)
	o.Enqueue(func() {
		wg.Done()
	})
	wg.Wait()
}

func (o *operations) pop() func() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.ops) == 0 {
		return nil
	}

	fn := o.ops[0]
	o.ops = o.ops[1:]
	return fn
}

func (o *operations) start() {
	fn := o.pop()
	for fn != nil {
		fn()
		fn = o.pop()
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.ops) == 0 {
		o.busy = false
		if o.busyHandler != nil {
			go o.busyHandler()
		}
		return
	}
	// either a new operation was enqueued while we
	// were busy, or an operation panicked
	go o.start()
}
