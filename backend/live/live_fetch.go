package live

import (
	"errors"
	"sync"
	"time"
)

var errLiveNotReady = errors.New("no live data yet")

type liveFetches struct {
	mu      sync.Mutex
	running *liveFetch
	last    time.Time
}

type liveFetch struct {
	done  chan struct{}
	table *liveTable
	err   error
}

func (h *liveHub) poll() (*liveTable, error) { return h.share(false) }

func (h *liveHub) current() (*liveTable, error) {
	if table := h.recent(); table != nil {
		return table, nil
	}
	return h.share(true)
}

func (h *liveHub) share(paced bool) (*liveTable, error) {
	var interval time.Duration
	if paced {
		interval = h.CurrentInterval()
	}
	fs := &h.fetches
	fs.mu.Lock()
	if f := fs.running; f != nil {
		fs.mu.Unlock()
		<-f.done
		return f.table, f.err
	}
	if paced && time.Since(fs.last) < interval {
		fs.mu.Unlock()
		if table := h.eng.currentLiveTable(); table != nil {
			return table, nil
		}
		return nil, errLiveNotReady
	}
	f := &liveFetch{done: make(chan struct{})}
	fs.running, fs.last = f, time.Now()
	fs.mu.Unlock()
	f.table, f.err = h.fetch()
	fs.mu.Lock()
	fs.running = nil
	fs.mu.Unlock()
	close(f.done)
	return f.table, f.err
}
