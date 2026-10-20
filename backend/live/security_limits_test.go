package live

import (
	"conexiuni-cluj/web"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (eng *Engine) withFreshLiveTable(t *testing.T) {
	t.Helper()
	eng.liveState.Lock()
	prev := eng.liveState.table
	eng.liveState.table = &liveTable{AsOf: time.Now()}
	eng.liveState.Unlock()
	t.Cleanup(func() {
		eng.liveState.Lock()
		eng.liveState.table = prev
		eng.liveState.Unlock()
	})
}

func resetLiveClients(t *testing.T) {
	t.Cleanup(func() {
		liveClients.Lock()
		liveClients.streams = map[string]int{}
		liveClients.Unlock()
	})
}

func TestLiveStreamsAreCappedPerClient(t *testing.T) {
	eng := newTestCity(t)
	eng.withFreshLiveTable(t)
	resetLiveClients(t)
	hub := &liveHub{subscribers: map[int64]*liveSubscriber{}, running: true, eng: eng}
	subscribe := func(client string) (int64, bool) {
		sub, id, ok := hub.Subscribe(client, true, []int{1}, nil)
		if ok {
			<-sub.ch
		}
		return id, ok
	}

	var ids []int64
	for range maxLiveStreamsPerClient {
		id, ok := subscribe("a")
		if !ok {
			t.Fatalf("stream %d refused below the cap", len(ids)+1)
		}
		ids = append(ids, id)
	}
	if _, ok := subscribe("a"); ok {
		t.Fatal("stream over the cap accepted")
	}
	if _, ok := subscribe("b"); !ok {
		t.Fatal("another client refused")
	}
	hub.Unsubscribe(ids[0])
	if _, ok := subscribe("a"); !ok {
		t.Fatal("closing a stream did not free its place")
	}
}

func TestLiveStreamCapCountsEveryCity(t *testing.T) {
	eng := newTestCity(t)
	eng.withFreshLiveTable(t)
	resetLiveClients(t)
	hubs := []*liveHub{
		{subscribers: map[int64]*liveSubscriber{}, running: true, eng: eng},
		{subscribers: map[int64]*liveSubscriber{}, running: true, eng: eng},
	}
	for i := range maxLiveStreamsPerClient {
		sub, _, ok := hubs[i%2].Subscribe("a", true, []int{1}, nil)
		if !ok {
			t.Fatalf("stream %d refused below the cap", i+1)
		}
		<-sub.ch
	}
	if _, _, ok := hubs[1].Subscribe("a", true, []int{1}, nil); ok {
		t.Fatal("another city's hub let the client past the cap")
	}
}

func TestRefusedLiveStreamTellsBrowsersWhenToRetry(t *testing.T) {
	eng := newTestCity(t)
	prev := eng.hub
	eng.hub = &liveHub{subscribers: map[int64]*liveSubscriber{}, running: true, eng: eng}
	liveClients.Lock()
	liveClients.streams["a"] = maxLiveStreamsPerClient
	liveClients.Unlock()
	t.Cleanup(func() {
		eng.hub = prev
		liveClients.Lock()
		delete(liveClients.streams, "a")
		liveClients.Unlock()
	})
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		web.SetClient(c, "a", "")
		return c.Next()
	})
	app.Get("/live", eng.ServeLive)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/live?stop_ids=1", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d, type %q: a browser would not retry", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if want := fmt.Sprintf("retry: %d\n\n", liveKeepAlive.Milliseconds()); string(body) != want {
		t.Fatalf("body %q, want %q", body, want)
	}
}
