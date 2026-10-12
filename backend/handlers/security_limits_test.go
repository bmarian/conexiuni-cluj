package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func withFreshLiveTable(t *testing.T) {
	t.Helper()
	liveState.Lock()
	prev := liveState.table
	liveState.table = &liveTable{AsOf: time.Now()}
	liveState.Unlock()
	t.Cleanup(func() {
		liveState.Lock()
		liveState.table = prev
		liveState.Unlock()
	})
}

func TestLiveStreamsAreCappedPerClient(t *testing.T) {
	withFreshLiveTable(t)
	hub := &liveHub{subscribers: map[int64]*liveSubscriber{}, perClient: map[string]int{}, running: true}
	subscribe := func(client string) (int64, bool) {
		sub, id, ok := hub.Subscribe(client, []int{1}, nil)
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

func TestPushSignupsAreBudgetedPerClient(t *testing.T) {
	withTestDB(t)
	pushSignupsMu.Lock()
	pushSignups = map[string]*clientCounter{}
	pushSignupsMu.Unlock()
	now := time.Now()

	known := pushSubscriber{endpoint: "https://fcm.googleapis.com/fcm/send/known", p256dh: "k", auth: "a", locale: "ro"}
	if err := savePushSubscription(known, nil); err != nil {
		t.Fatal(err)
	}
	for i := range maxPushSignupsPerClient {
		if ok, err := admitPushEndpoint("a", "https://fcm.googleapis.com/fcm/send/new", now); err != nil || !ok {
			t.Fatalf("sign-up %d refused below the budget: %v", i+1, err)
		}
	}
	if ok, _ := admitPushEndpoint("a", "https://fcm.googleapis.com/fcm/send/new", now); ok {
		t.Fatal("sign-up over the budget accepted")
	}
	if ok, _ := admitPushEndpoint("a", known.endpoint, now); !ok {
		t.Fatal("re-saving a known endpoint was refused")
	}
	if ok, _ := admitPushEndpoint("b", "https://fcm.googleapis.com/fcm/send/new", now); !ok {
		t.Fatal("another client refused")
	}
	if ok, _ := admitPushEndpoint("a", "https://fcm.googleapis.com/fcm/send/new", now.Add(pushSignupWindow+time.Second)); !ok {
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
