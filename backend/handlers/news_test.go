package handlers

import (
	"conexiuni-cluj/sources"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestUpdatesMergeNewsAndFollowedChanges(t *testing.T) {
	city := newTestCity(t)
	now := time.Now()
	if err := city.storeNews([]sources.NewsItem{{Title: "Gone", URL: "https://op.ro/gone", At: now}}); err != nil {
		t.Fatal(err)
	}
	if err := city.storeNews([]sources.NewsItem{
		{Title: "Deviation", URL: "https://op.ro/a", At: now.Add(-time.Hour)},
		{Title: "Old", URL: "https://op.ro/old", At: now.Add(-updateRetention - time.Hour)},
		{Title: "Tickets", URL: "https://op.ro/b", At: now.Add(-3 * time.Hour)},
	}); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"35", "24"} {
		if _, err := city.DB.Exec(`INSERT INTO route_changes (route_short_name, source, changes, signature, detected_at) VALUES (?, 'timetable', '[]', ?, ?)`,
			line, line, now.Add(-2*time.Hour).UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	app := fiber.New()
	app.Get("/updates", city.GetUpdates)

	for query, want := range map[string][]string{
		"/updates?routes=35": {"news Deviation", "change 35", "news Tickets"},
		"/updates":           {"news Deviation", "news Tickets"},
	} {
		resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, query, nil))
		if err != nil {
			t.Fatal(err)
		}
		var updates []Update
		if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, u := range updates {
			if u.News != nil {
				got = append(got, u.Kind+" "+u.News.Title)
			} else {
				got = append(got, u.Kind+" "+u.Change.RouteShortName)
			}
		}
		if len(got) != len(want) {
			t.Fatalf("%s = %v, want %v", query, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s = %v, want %v", query, got, want)
				break
			}
		}
	}
}
