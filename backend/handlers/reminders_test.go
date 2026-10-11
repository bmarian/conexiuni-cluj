package handlers

import (
	"bytes"
	"conexiuni-cluj/database"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

var cluj = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		panic(err)
	}
	return loc
}()

func clujTime(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.ParseInLocation("2006-01-02 15:04", value, cluj)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func reminder(id string, days []int, date string, slots map[string]int) pushReminder {
	return pushReminder{
		ID: id, StopID: 120, StopName: "P-ța Mihai Viteazul Sud", RouteID: 57, TripID: "57_0", Line: "57",
		Destination: "Gheorgheni", Days: days, Date: date, Slots: slots,
	}
}

func TestValidReminder(t *testing.T) {
	ok := reminder("a-1", []int{1, 2, 3, 4, 5}, "", map[string]int{"weekday": 1385})
	if !validReminder(ok) {
		t.Fatal("a plain weekday reminder was refused")
	}
	once := reminder("a-2", nil, "2026-10-09", map[string]int{"weekday": 1445})
	if !validReminder(once) {
		t.Fatal("a one-off past midnight was refused")
	}
	bad := map[string]func(r *pushReminder){
		"empty id":           func(r *pushReminder) { r.ID = "" },
		"odd id":             func(r *pushReminder) { r.ID = "a b" },
		"no stop":            func(r *pushReminder) { r.StopID = 0 },
		"no route":           func(r *pushReminder) { r.RouteID = -1 },
		"trip with a slash":  func(r *pushReminder) { r.TripID = "57/0" },
		"trip with a query":  func(r *pushReminder) { r.TripID = "57_0&x=1" },
		"no line":            func(r *pushReminder) { r.Line = "" },
		"long stop name":     func(r *pushReminder) { r.StopName = strings.Repeat("x", 121) },
		"no stop name":       func(r *pushReminder) { r.StopName = "" },
		"no slots":           func(r *pushReminder) { r.Slots = nil },
		"unknown day type":   func(r *pushReminder) { r.Slots = map[string]int{"holiday": 600} },
		"negative slot":      func(r *pushReminder) { r.Slots = map[string]int{"weekday": -1} },
		"slot too late":      func(r *pushReminder) { r.Slots = map[string]int{"weekday": 1440 + 180} },
		"no days, no date":   func(r *pushReminder) { r.Days = nil },
		"day zero":           func(r *pushReminder) { r.Days = []int{0} },
		"day eight":          func(r *pushReminder) { r.Days = []int{8} },
		"repeated day":       func(r *pushReminder) { r.Days = []int{1, 1} },
		"bad date":           func(r *pushReminder) { r.Days = nil; r.Date = "2026-13-01" },
		"both days and date": func(r *pushReminder) { r.Date = "2026-10-09" },
	}
	for name, change := range bad {
		r := ok
		r.Days = append([]int(nil), ok.Days...)
		change(&r)
		if validReminder(r) {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNextReminderBus(t *testing.T) {
	every := []int{1, 2, 3, 4, 5, 6, 7}
	slots := map[string]int{"weekday": 23*60 + 42, "saturday": 23 * 60}
	cases := []struct {
		name  string
		r     pushReminder
		after string
		want  string
	}{
		// 2026-10-09 is a Friday.
		{"today", reminder("x", every, "", slots), "2026-10-09 20:00", "2026-10-09 23:42"},
		{"tomorrow's Saturday bus", reminder("x", every, "", slots), "2026-10-09 23:50", "2026-10-10 23:00"},
		{"no Sunday bus", reminder("x", every, "", slots), "2026-10-10 23:30", "2026-10-12 23:42"},
		{"Mondays only", reminder("x", []int{1}, "", slots), "2026-10-09 20:00", "2026-10-12 23:42"},
		{"late bus", reminder("x", []int{5}, "", map[string]int{"weekday": 24*60 + 5}), "2026-10-09 23:00", "2026-10-10 00:05"},
		{"late bus, after midnight", reminder("x", []int{5}, "", map[string]int{"weekday": 24*60 + 5}), "2026-10-10 00:01", "2026-10-10 00:05"},
		{"one-off", reminder("x", nil, "2026-10-09", map[string]int{"weekday": 24*60 + 5}), "2026-10-09 08:00", "2026-10-10 00:05"},
		// Clocks go back on 2026-10-25.
		{"across DST", reminder("x", []int{6}, "", map[string]int{"saturday": 24*60 + 30}), "2026-10-24 20:00", "2026-10-25 00:30"},
	}
	for _, c := range cases {
		got, ok := nextReminderBus(c.r, clujTime(t, c.after), cluj)
		if !ok || !got.Equal(clujTime(t, c.want)) {
			t.Errorf("%s: got %v (%v), want %s", c.name, got.In(cluj), ok, c.want)
		}
	}
	if _, ok := nextReminderBus(reminder("x", nil, "2026-10-09", map[string]int{"weekday": 24*60 + 5}), clujTime(t, "2026-10-10 00:06"), cluj); ok {
		t.Error("a one-off came back after its bus")
	}
	if _, ok := nextReminderBus(reminder("x", every, "", map[string]int{"sunday": 600}), clujTime(t, "2026-10-12 08:00"), cluj); !ok {
		t.Error("a Sunday-only bus wasn't found within the week")
	}
}

func syncRequest(t *testing.T, endpoint string, reminders []pushReminder) *http.Request {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"endpoint":  endpoint,
		"keys":      map[string]string{"p256dh": base64.RawURLEncoding.EncodeToString(append([]byte{4}, bytes.Repeat([]byte{1}, 64)...)), "auth": base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 16))},
		"locale":    "en",
		"reminders": reminders,
	})
	req := httptest.NewRequest("POST", "/api/push/reminders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func storedReminders(t *testing.T, endpoint string) map[string]int64 {
	t.Helper()
	rows, err := database.DB.Query(`SELECT id, last_fired_at FROM push_reminders WHERE endpoint = ?`, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var at int64
		_ = rows.Scan(&id, &at)
		out[id] = at
	}
	return out
}

func TestSyncRemindersReplacesAndKeepsWhenEachFired(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	app := fiber.New()
	app.Post("/api/push/reminders", SyncReminders)
	endpoint := "https://fcm.googleapis.com/fcm/send/test"
	weekday := reminder("a", []int{1, 2, 3, 4, 5}, "", map[string]int{"weekday": 1385})

	resp, err := app.Test(syncRequest(t, endpoint, []pushReminder{weekday, reminder("b", []int{6}, "", map[string]int{"saturday": 600})}))
	if err != nil || resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("sync: %v %v", err, resp.StatusCode)
	}
	if got := storedReminders(t, endpoint); len(got) != 2 {
		t.Fatalf("stored %v", got)
	}
	if _, err := database.DB.Exec(`UPDATE push_reminders SET last_fired_at = 42 WHERE id = 'a'`); err != nil {
		t.Fatal(err)
	}
	resp, _ = app.Test(syncRequest(t, endpoint, []pushReminder{weekday}))
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("re-sync: %v", resp.StatusCode)
	}
	if got := storedReminders(t, endpoint); len(got) != 1 || got["a"] != 42 {
		t.Fatalf("after re-sync: %v", got)
	}
	followLines(t, pushSubscriber{endpoint: endpoint, p256dh: "x", auth: "y", locale: "en"}, "25")
	resp, _ = app.Test(syncRequest(t, endpoint, nil))
	if resp.StatusCode != fiber.StatusNoContent || len(storedReminders(t, endpoint)) != 0 {
		t.Fatalf("clear: %v %v", resp.StatusCode, storedReminders(t, endpoint))
	}
	var routes int
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM push_subscription_routes WHERE endpoint = ?`, endpoint).Scan(&routes)
	if routes != 1 {
		t.Fatalf("line alerts went with the reminders: %d", routes)
	}
	_, _ = app.Test(syncRequest(t, endpoint, []pushReminder{weekday}))
	if err := deletePushSubscription(endpoint); err != nil {
		t.Fatal(err)
	}
	if got := storedReminders(t, endpoint); len(got) != 0 {
		t.Fatalf("reminders outlived their subscription: %v", got)
	}
}

func TestSyncRemindersRefusesBadRequests(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	app := fiber.New()
	app.Post("/api/push/reminders", SyncReminders)
	ok := reminder("a", []int{1}, "", map[string]int{"weekday": 600})
	many := make([]pushReminder, maxRemindersPerSub+1)
	for i := range many {
		many[i] = ok
		many[i].ID = strings.Repeat("a", i+1)
	}
	cases := map[string]*http.Request{
		"foreign endpoint": syncRequest(t, "https://evil.example/push", []pushReminder{ok}),
		"too many":         syncRequest(t, "https://fcm.googleapis.com/fcm/send/x", many),
		"duplicate ids":    syncRequest(t, "https://fcm.googleapis.com/fcm/send/x", []pushReminder{ok, ok}),
		"invalid reminder": syncRequest(t, "https://fcm.googleapis.com/fcm/send/x", []pushReminder{reminder("a", nil, "", map[string]int{"weekday": 600})}),
	}
	for name, req := range cases {
		resp, _ := app.Test(req)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Errorf("%s: %d", name, resp.StatusCode)
		}
	}
	cross := syncRequest(t, "https://fcm.googleapis.com/fcm/send/x", []pushReminder{ok})
	cross.Header.Set("Sec-Fetch-Site", "cross-site")
	cross.Header.Set("Origin", "https://evil.example")
	if resp, _ := app.Test(cross); resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("cross-site: %d", resp.StatusCode)
	}
	pushCfg = nil
	if resp, _ := app.Test(syncRequest(t, "https://fcm.googleapis.com/fcm/send/x", []pushReminder{ok})); resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Errorf("without push: %d", resp.StatusCode)
	}
}

func TestFireDueRemindersSendsOncePerBus(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	client := newTestPushClient(t, "en", http.StatusCreated)
	every := []int{1, 2, 3, 4, 5, 6, 7}
	if err := saveReminders(client.sub, []pushReminder{
		reminder("night", every, "", map[string]int{"weekday": 22*60 + 50, "saturday": 22*60 + 30}),
	}); err != nil {
		t.Fatal(err)
	}

	fireDueReminders(clujTime(t, "2026-10-09 22:25"), cluj)
	if got := client.received(); len(got) != 0 {
		t.Fatalf("sent 25 min early: %v", got)
	}
	fireDueReminders(clujTime(t, "2026-10-09 22:31"), cluj)
	fireDueReminders(clujTime(t, "2026-10-09 22:40"), cluj)
	got := client.received()
	if len(got) != 1 {
		t.Fatalf("want one push for the 22:50 bus, got %v", got)
	}
	bus := clujTime(t, "2026-10-09 22:50").UnixMilli()
	want := pushMessage{
		Title: "Leave now", Body: "57 in 19 min · P-ța Mihai Viteazul Sud",
		URL: "/stop/120?follow=57_0&at=" + strconv.FormatInt(bus, 10), Tag: "reminder-night", Timestamp: clujTime(t, "2026-10-09 22:31").UnixMilli(),
	}
	if got[0] != want {
		t.Fatalf("push:\n got %+v\nwant %+v", got[0], want)
	}
	fireDueReminders(clujTime(t, "2026-10-09 22:55"), cluj)
	fireDueReminders(clujTime(t, "2026-10-10 22:12"), cluj)
	if got := client.received(); len(got) != 2 || got[1].Body != "57 in 18 min · P-ța Mihai Viteazul Sud" {
		t.Fatalf("Saturday: %v", got)
	}
}

func TestFireDueRemindersInRomanianAndForgetsPastOneOffs(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	client := newTestPushClient(t, "ro", http.StatusCreated)
	if err := saveReminders(client.sub, []pushReminder{
		reminder("once", nil, "2026-10-09", map[string]int{"weekday": 24*60 + 5}),
	}); err != nil {
		t.Fatal(err)
	}
	fireDueReminders(clujTime(t, "2026-10-09 23:50"), cluj)
	got := client.received()
	if len(got) != 1 || got[0].Title != "Pleacă acum" || got[0].Body != "57 în 15 min · P-ța Mihai Viteazul Sud" {
		t.Fatalf("push: %v", got)
	}
	fireDueReminders(clujTime(t, "2026-10-10 00:03"), cluj)
	if len(storedReminders(t, client.sub.endpoint)) != 1 {
		t.Fatal("a one-off went before its bus")
	}
	fireDueReminders(clujTime(t, "2026-10-10 00:10"), cluj)
	if got := storedReminders(t, client.sub.endpoint); len(got) != 0 {
		t.Fatalf("a past one-off stayed: %v", got)
	}
	if len(client.received()) != 1 {
		t.Fatal("a one-off fired twice")
	}
}

func TestFireDueRemindersSkipsMissedBusesAndDropsDeadSubscriptions(t *testing.T) {
	withTestDB(t)
	withTestPush(t)
	gone := newTestPushClient(t, "en", http.StatusGone)
	if err := saveReminders(gone.sub, []pushReminder{reminder("a", []int{5}, "", map[string]int{"weekday": 600})}); err != nil {
		t.Fatal(err)
	}
	fireDueReminders(clujTime(t, "2026-10-09 10:05"), cluj)
	if len(gone.received()) != 0 {
		t.Fatal("sent for a bus that had already gone")
	}
	fireDueReminders(clujTime(t, "2026-10-16 09:45"), cluj)
	if len(gone.received()) != 1 {
		t.Fatal("next week's bus wasn't sent")
	}
	if got := storedReminders(t, gone.sub.endpoint); len(got) != 0 {
		t.Fatalf("reminders kept for a dead subscription: %v", got)
	}
}
