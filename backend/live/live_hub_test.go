package live

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/models"
	"sync"
	"testing"
	"time"
)

// Holds every fetch until released, so fetches that overlap are all counted.
type heldFeed struct {
	started chan struct{}
	release chan struct{}
}

func (f *heldFeed) Vehicles() ([]models.Vehicle, error) {
	f.started <- struct{}{}
	<-f.release
	return nil, nil
}

func (f *heldFeed) Budget() feeds.Budget { return feeds.Budget{} }

func TestTheFirstViewerCostsOneFetch(t *testing.T) {
	eng := newTestCity(t)
	feed := &heldFeed{started: make(chan struct{}, 4), release: make(chan struct{})}
	eng.Realtime = feed
	eng.Options.VehicleInterval = VehicleIntervalConfig{MinInterval: time.Hour, MaxInterval: time.Hour}
	release := sync.OnceFunc(func() { close(feed.release) })
	r := jobs.New()
	eng.Runner = func() *jobs.Runner { return r }
	t.Cleanup(func() {
		release()
		r.Stop()
		r.Wait()
	})
	resetLiveClients(t)
	firstTable := func(sub *liveSubscriber) {
		t.Helper()
		select {
		case <-sub.ch:
		case <-time.After(5 * time.Second):
			t.Fatal("no first table")
		}
	}

	first, _, ok := eng.hub.Subscribe("a", true, []int{1}, nil)
	if !ok {
		t.Fatal("refused")
	}
	<-feed.started
	select {
	case <-feed.started:
		t.Fatal("a cold hub fetched twice for its first viewer")
	case <-time.After(200 * time.Millisecond):
	}
	release()
	firstTable(first)

	second, _, _ := eng.hub.Subscribe("b", true, []int{1}, nil)
	firstTable(second)
	if n := len(feed.started); n != 0 {
		t.Fatalf("a second viewer a moment later cost %d fetches", n)
	}
}

func TestOnDemandAnswersShareAndPaceFetches(t *testing.T) {
	eng := newTestCity(t)
	feed := &heldFeed{started: make(chan struct{}, 32), release: make(chan struct{})}
	eng.Realtime = feed
	eng.Options.VehicleInterval = VehicleIntervalConfig{MinInterval: time.Hour, MaxInterval: time.Hour}
	r := jobs.New()
	eng.Runner = func() *jobs.Runner { return r }
	t.Cleanup(func() {
		r.Stop()
		r.Wait()
	})

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := eng.hub.current(); err != nil {
				t.Errorf("current: %v", err)
			}
		}()
	}
	<-feed.started
	select {
	case <-feed.started:
		t.Fatal("20 answers at once fetched more than once")
	case <-time.After(200 * time.Millisecond):
	}
	close(feed.release)
	wg.Wait()

	stale := &liveTable{AsOf: time.Now().Add(-time.Hour)}
	eng.liveState.Lock()
	eng.liveState.table = stale
	eng.liveState.Unlock()
	if table, err := eng.hub.current(); err != nil || table != stale {
		t.Fatalf("within the pace: table %v, err %v; want the last table", table, err)
	}
	if n := len(feed.started); n != 0 {
		t.Fatalf("an answer within the pace cost %d fetches", n)
	}

	eng.hub.fetches.mu.Lock()
	eng.hub.fetches.last = time.Now().Add(-2 * time.Hour)
	eng.hub.fetches.mu.Unlock()
	if _, err := eng.hub.current(); err != nil {
		t.Fatal(err)
	}
	if n := len(feed.started); n != 1 {
		t.Fatalf("an answer past the pace cost %d fetches, want 1", n)
	}
}
