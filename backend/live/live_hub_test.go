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
