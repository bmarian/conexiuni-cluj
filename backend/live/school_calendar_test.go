package live

import (
	"bytes"
	"conexiuni-cluj/calendar"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

type fakeSchoolCalendar struct {
	years []calendar.SchoolYear
	err   error
}

func (f *fakeSchoolCalendar) SchoolYears() ([]calendar.SchoolYear, error) { return f.years, f.err }
func (f *fakeSchoolCalendar) Cadence() time.Duration                      { return time.Hour }
func (f *fakeSchoolCalendar) Page() string                                { return "https://example.test/judet/cluj" }

func TestSchoolCalendarSync(t *testing.T) {
	eng := newTestCity(t)
	eng.withSeedCalendar(t)
	relearned := 0
	prevRelearn := relearnAfterCalendarChange
	relearnAfterCalendarChange = func(*Engine, time.Time) { relearned++ }
	t.Cleanup(func() { relearnAfterCalendarChange = prevRelearn })

	builtIn := calendar.Seed()[0].Normalized()
	groupB := calendar.Seed()[0].Normalized()
	for i, b := range groupB.Breaks {
		if b.Start == "2027-02-15" {
			groupB.Breaks[i] = calendar.Range{Name: "Vacanța intersemestrială", Start: "2027-02-22", End: "2027-02-28"}
		}
	}
	invalid := calendar.Estimate(builtIn)
	invalid.Start = "2028-04-01"
	source := &fakeSchoolCalendar{years: []calendar.SchoolYear{groupB, invalid}}
	eng.SchoolCalendar = source
	now := eng.clujAt("2026-10-26", 12)
	skiWeek := eng.clujAt("2027-02-23", 12)

	app := fiber.New()
	RegisterAdminCalendar(app.Group("/api/admin"), func(fiber.Ctx) *Engine { return eng }, func(c fiber.Ctx) error { return c.Next() })
	put := func(y calendar.SchoolYear) adminCalendarView {
		t.Helper()
		body, _ := json.Marshal(y)
		req := httptest.NewRequest("PUT", "/api/admin/calendar", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("save: %v %v", resp.StatusCode, err)
		}
		var view adminCalendarView
		if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
			t.Fatal(err)
		}
		return view
	}
	year2026 := func(view adminCalendarView) adminCalendarYear {
		t.Helper()
		for _, y := range view.Years {
			if y.Year == 2026 {
				return y
			}
		}
		t.Fatalf("no 2026 in %+v", view.Years)
		return adminCalendarYear{}
	}
	sync := func() {
		t.Helper()
		if err := eng.syncSchoolCalendar(now); err != nil {
			t.Fatal(err)
		}
	}

	sync()
	if got := eng.profileDay(skiWeek); got != calendar.WeekdayVacation || relearned != 1 {
		t.Fatalf("after the first read: %s on 23 Feb, relearned %d times", got, relearned)
	}
	if eng.currentCalendar().HasYear(2027) {
		t.Fatal("an invalid year from the feed was kept")
	}
	view, err := eng.buildAdminCalendar(now)
	if err != nil {
		t.Fatal(err)
	}
	if y := year2026(view); y.Status != "synced" || view.Sync == nil || view.Sync.CheckedAt != now.Unix() || view.Sync.Error != "" {
		t.Fatalf("status %q, sync %+v", y.Status, view.Sync)
	}

	sync()
	if relearned != 1 {
		t.Fatalf("an unchanged feed relearned (%d)", relearned)
	}

	view = put(builtIn)
	if y := year2026(view); y.Status != "saved" || y.Synced == nil || !y.Synced.Equal(groupB) {
		t.Fatalf("a year set by hand: status %q, synced %+v", y.Status, y.Synced)
	}
	if got := eng.profileDay(skiWeek); got != calendar.Weekday {
		t.Fatalf("the hand-set year lost to the feed: %s", got)
	}

	view = put(groupB)
	if y := year2026(view); y.Status != "synced" || y.Synced != nil {
		t.Fatalf("saving the feed's dates: status %q", y.Status)
	}
	if saved, _ := eng.savedSchoolYears(); len(saved) != 0 {
		t.Fatalf("saving the feed's dates kept a hand-set year: %+v", saved)
	}

	data, _ := json.Marshal(groupB)
	if err := eng.SaveSchoolYear(2026, string(data), now); err != nil {
		t.Fatal(err)
	}
	sync()
	if saved, _ := eng.savedSchoolYears(); len(saved) != 0 {
		t.Fatalf("a hand-set year matching the feed was kept: %+v", saved)
	}

	source.err = errors.New("feed down")
	if err := eng.syncSchoolCalendar(now.Add(time.Hour)); err == nil {
		t.Fatal("a failed read reported no error")
	}
	view, _ = eng.buildAdminCalendar(now)
	if view.Sync.Error != "feed down" || view.Sync.CheckedAt != now.Unix() || eng.profileDay(skiWeek) != calendar.WeekdayVacation {
		t.Fatalf("after a failed read: sync %+v, 23 Feb %s", view.Sync, eng.profileDay(skiWeek))
	}
}
