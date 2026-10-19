package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (city *City) withFreshLiveTable(t *testing.T) {
	t.Helper()
	city.liveState.Lock()
	prev := city.liveState.table
	city.liveState.table = &liveTable{AsOf: time.Now()}
	city.liveState.Unlock()
	t.Cleanup(func() {
		city.liveState.Lock()
		city.liveState.table = prev
		city.liveState.Unlock()
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
	city := newTestCity(t)
	city.withFreshLiveTable(t)
	resetLiveClients(t)
	hub := &liveHub{subscribers: map[int64]*liveSubscriber{}, running: true, city: city}
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
	city := newTestCity(t)
	city.withFreshLiveTable(t)
	resetLiveClients(t)
	hubs := []*liveHub{
		{subscribers: map[int64]*liveSubscriber{}, running: true, city: city},
		{subscribers: map[int64]*liveSubscriber{}, running: true, city: city},
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

func TestPlansRunOneAtATimePerClient(t *testing.T) {
	release, ok := acquirePlanSlot("a")
	if !ok {
		t.Fatal("first plan refused")
	}
	second := make(chan func())
	go func() {
		r, _ := acquirePlanSlot("a")
		second <- r
	}()
	if cap(planSlots) > 1 {
		r, ok := acquirePlanSlot("b")
		if !ok {
			t.Fatal("another client was held up")
		}
		r()
	}
	select {
	case <-second:
		t.Fatal("a second plan of the same client ran alongside the first")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case r := <-second:
		r()
	case <-time.After(time.Second):
		t.Fatal("the client's next plan never started")
	}
	planTurns.Lock()
	left := len(planTurns.byClient)
	planTurns.Unlock()
	if left != 0 {
		t.Fatalf("%d clients still listed after their plans ended", left)
	}
}

func TestPushSignupsAreBudgetedPerClient(t *testing.T) {
	city := newTestCity(t)
	pushSignupsMu.Lock()
	pushSignups = map[string]*clientCounter{}
	pushSignupsMu.Unlock()
	now := time.Now()

	known := pushSubscriber{endpoint: "https://fcm.googleapis.com/fcm/send/known", p256dh: "k", auth: "a", locale: "ro"}
	if err := city.savePushSubscription(known, nil); err != nil {
		t.Fatal(err)
	}
	for i := range maxPushSignupsPerClient {
		if ok, err := city.admitPushEndpoint("a", "https://fcm.googleapis.com/fcm/send/new", now); err != nil || !ok {
			t.Fatalf("sign-up %d refused below the budget: %v", i+1, err)
		}
	}
	if ok, _ := city.admitPushEndpoint("a", "https://fcm.googleapis.com/fcm/send/new", now); ok {
		t.Fatal("sign-up over the budget accepted")
	}
	if ok, _ := city.admitPushEndpoint("a", known.endpoint, now); !ok {
		t.Fatal("re-saving a known endpoint was refused")
	}
	if ok, _ := city.admitPushEndpoint("b", "https://fcm.googleapis.com/fcm/send/new", now); !ok {
		t.Fatal("another client refused")
	}
	if ok, _ := city.admitPushEndpoint("a", "https://fcm.googleapis.com/fcm/send/new", now.Add(pushSignupWindow+time.Second)); !ok {
		t.Fatal("budget did not renew after the window")
	}
}

func TestLivePlanDay(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Bucharest")
	morning := time.Date(2026, 10, 12, 9, 0, 0, 0, loc)
	evening := time.Date(2026, 10, 12, 21, 0, 0, 0, loc)
	cases := []struct {
		key  string
		now  time.Time
		want bool
	}{
		{"2026-10-12", morning, true},
		{"2026-10-13", morning, false},
		{"2026-10-13", evening, true},
		{"2026-10-11", morning, false},
		{"9999-01-01", morning, false},
	}
	for _, tc := range cases {
		if got := livePlanDay(tc.key, tc.now); got != tc.want {
			t.Errorf("livePlanDay(%s, %s) = %v, want %v", tc.key, tc.now.Format("15:04"), got, tc.want)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders)
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Strict-Transport-Security", "X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
}

func TestRefusedLiveStreamTellsBrowsersWhenToRetry(t *testing.T) {
	city := newTestCity(t)
	prev := city.hub
	city.hub = &liveHub{subscribers: map[int64]*liveSubscriber{}, running: true, city: city}
	liveClients.Lock()
	liveClients.streams["a"] = maxLiveStreamsPerClient
	liveClients.Unlock()
	t.Cleanup(func() {
		city.hub = prev
		liveClients.Lock()
		delete(liveClients.streams, "a")
		liveClients.Unlock()
	})
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(clientHashLocalKey, "a")
		return c.Next()
	})
	app.Get("/live", city.ServeLive)

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

func TestIPv6ClientsCountPerSlash64(t *testing.T) {
	same := []string{"2a02:2f0c:8a12:3400::1", "2a02:2f0c:8a12:3400:abcd:1234:5678:9abc"}
	if clientKey(same[0]) != clientKey(same[1]) {
		t.Errorf("one /64 gives two clients: %s, %s", clientKey(same[0]), clientKey(same[1]))
	}
	if clientKey("2a02:2f0c:8a12:3401::1") == clientKey(same[0]) {
		t.Error("two /64s give one client")
	}
	if got := clientKey("188.24.64.220"); got != "188.24.64.220" {
		t.Errorf("IPv4 changed to %s", got)
	}
}
