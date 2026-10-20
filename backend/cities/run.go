package cities

import (
	"conexiuni-cluj/jobs"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
)

type cityRun struct {
	lifecycle sync.Mutex
	enabled   atomic.Bool
	current   atomic.Pointer[jobs.Runner]
	startedAt atomic.Int64
}

func (c *City) Enabled() bool { return c.run.enabled.Load() }

// When the background work last started.
func (c *City) StartedAt() time.Time { return time.Unix(0, c.run.startedAt.Load()) }

func (c *City) running() *jobs.Runner {
	if r := c.run.current.Load(); r != nil {
		return r
	}
	return jobs.None()
}

// Start and Stop bring the background work in line with Enabled, whatever order they run in.
func (c *City) Start() { c.syncRun() }
func (c *City) Stop()  { c.syncRun() }

func (c *City) syncRun() {
	c.run.lifecycle.Lock()
	defer c.run.lifecycle.Unlock()
	current := c.running()
	switch {
	case c.Enabled() && current.Stopped():
		current.Wait()
		r := jobs.New()
		c.run.current.Store(r)
		c.run.startedAt.Store(time.Now().UnixNano())
		c.start(r)
	case !c.Enabled() && !current.Stopped():
		current.Stop()
		if err := c.FlushStats(); err != nil {
			c.Logf("stats flush failed: %v", err)
		}
		c.Logf("stopped")
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
