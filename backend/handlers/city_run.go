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
}

func (r *runner) Go(fn func()) {
	r.tasks.Add(1)
	go func() {
		defer r.tasks.Done()
		fn()
	}()
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
		current.cancel()
		current.tasks.Wait()
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
