// Package jobs runs a city's background work and stops all of it together.
package jobs

import (
	"context"
	"sync"
	"time"
)

// One start of a city's background work, ended by Stop.
type Runner struct {
	ctx    context.Context
	cancel context.CancelFunc
	tasks  sync.WaitGroup
	joins  sync.RWMutex
}

func New() *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{ctx: ctx, cancel: cancel}
}

var none = func() *Runner {
	r := New()
	r.cancel()
	return r
}()

// None is a runner that is already stopped.
func None() *Runner { return none }

// False once the runner is stopped: fn never runs.
func (r *Runner) Go(fn func()) bool {
	r.joins.RLock()
	defer r.joins.RUnlock()
	if r.Stopped() {
		return false
	}
	r.tasks.Add(1)
	go func() {
		defer r.tasks.Done()
		fn()
	}()
	return true
}

// Stop cancels the work and waits for every job to return.
func (r *Runner) Stop() {
	r.joins.Lock()
	r.cancel()
	r.joins.Unlock()
	r.tasks.Wait()
}

// Wait returns once every job has returned.
func (r *Runner) Wait() { r.tasks.Wait() }

// False once the runner is stopped.
func (r *Runner) Sleep(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.ctx.Done():
		return false
	}
}

func (r *Runner) Stopped() bool { return r.ctx.Err() != nil }

// Done is closed once the runner is stopped.
func (r *Runner) Done() <-chan struct{} { return r.ctx.Done() }

func (r *Runner) Context() context.Context { return r.ctx }
