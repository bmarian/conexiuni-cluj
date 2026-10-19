package handlers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

// One start of a city's background work, ended by its cancel.
type runner struct {
	ctx    context.Context
	cancel context.CancelFunc
	tasks  *sync.WaitGroup
	joins  sync.RWMutex
}

// False once the runner is stopped: fn never runs.
func (r *runner) Go(fn func()) bool {
	r.joins.RLock()
	defer r.joins.RUnlock()
	if r.stopped() {
		return false
	}
	r.tasks.Add(1)
	go func() {
		defer r.tasks.Done()
		fn()
	}()
	return true
}

func (r *runner) stop() {
	r.joins.Lock()
	r.cancel()
	r.joins.Unlock()
	r.tasks.Wait()
}

// False once the city is stopped.
func (r *runner) Sleep(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.ctx.Done():
		return false
	}
}

func (r *runner) stopped() bool { return r.ctx.Err() != nil }

var stoppedRunner = func() *runner {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return &runner{ctx: ctx, cancel: cancel, tasks: &sync.WaitGroup{}}
}()

type cityRun struct {
	lifecycle sync.Mutex
	enabled   atomic.Bool
	current   atomic.Pointer[runner]
}

func (city *City) Enabled() bool { return city.run.enabled.Load() }

func (city *City) running() *runner {
	if r := city.run.current.Load(); r != nil {
		return r
	}
	return stoppedRunner
}

// Start and Stop bring the background work in line with Enabled, whatever order they run in.
func (city *City) Start() { city.syncRun() }
func (city *City) Stop()  { city.syncRun() }

func (city *City) syncRun() {
	city.run.lifecycle.Lock()
	defer city.run.lifecycle.Unlock()
	current := city.running()
	switch {
	case city.Enabled() && current.stopped():
		current.tasks.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		r := &runner{ctx: ctx, cancel: cancel, tasks: &sync.WaitGroup{}}
		city.run.current.Store(r)
		city.start(r)
	case !city.Enabled() && !current.stopped():
		current.stop()
		if err := city.FlushStats(); err != nil {
			city.log("stats flush failed: %v", err)
		}
		city.log("stopped")
	}
}

// Answers for a disabled city as for one that does not exist.
func (city *City) OnlyEnabled(missing fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		if city.Enabled() {
			return c.Next()
		}
		return missing(c)
	}
}
