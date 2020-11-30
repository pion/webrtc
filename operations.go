package webrtc

import (
	"context"
	"sync"
)

// Operation is a function
type operation func(ctx context.Context)

// Operations is a task executor.
type operations struct {
	mu   sync.Mutex
	busy bool
	ops  []operation
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
		go o.start(context.Background())
	}
}

// IsEmpty checks if there are tasks in the queue
func (o *operations) IsEmpty() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.ops) == 0
}

// Done blocks until all currently enqueued operations are finished executing.
// For more complex synchronization, use Enqueue directly.
func (o *operations) Done() {
	var wg sync.WaitGroup
	wg.Add(1)
	o.Enqueue(func(_ context.Context) {
		wg.Done()
	})
	wg.Wait()
}

func (o *operations) pop() func(context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.ops) == 0 {
		return nil
	}

	fn := o.ops[0]
	o.ops = o.ops[1:]
	return fn
}

func (o *operations) start(ctx context.Context) {
	defer func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if len(o.ops) == 0 {
			o.busy = false
			return
		}
		// either a new operation was enqueued while we
		// were busy, or an operation panicked
		go o.start(ctx)
	}()

	fn := o.pop()
	for fn != nil {
		fn(ctx)
		fn = o.pop()
	}
}
