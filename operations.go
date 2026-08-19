package main

import (
	"context"
	"sync"
	"time"
)

// opRegistry tracks the long-running, user-initiated operations (recursive
// search, prefix statistics, capability probing) so the frontend can abandon one
// that is taking too long. Without it those calls run to their timeout with no
// way out, holding a connection and a spinner the whole time.
type opRegistry struct {
	mu  sync.Mutex
	ops map[string]context.CancelFunc
}

func newOpRegistry() *opRegistry {
	return &opRegistry{ops: map[string]context.CancelFunc{}}
}

// begin returns a context for the operation plus a release function that must be
// deferred. An empty id yields a plain timeout context that cannot be cancelled
// from the UI, which keeps callers that do not care simple.
func (r *opRegistry) begin(id string, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	if id == "" {
		return ctx, cancel
	}
	r.mu.Lock()
	// A re-used id supersedes the operation it names.
	if prev, ok := r.ops[id]; ok {
		prev()
	}
	r.ops[id] = cancel
	r.mu.Unlock()

	return ctx, func() {
		r.mu.Lock()
		delete(r.ops, id)
		r.mu.Unlock()
		cancel()
	}
}

// cancel aborts the operation with the given id, reporting whether one was
// running under it.
func (r *opRegistry) cancel(id string) bool {
	r.mu.Lock()
	cancel, ok := r.ops[id]
	if ok {
		delete(r.ops, id)
	}
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// count reports how many operations are in flight (used by tests).
func (r *opRegistry) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ops)
}
