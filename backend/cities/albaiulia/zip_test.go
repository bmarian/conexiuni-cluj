package albaiulia

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds/gtfs"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"slices"
	"testing"
	"time"
)

type ids map[string]int

func (m ids) ExternalID(kind, external string) (int, error) {
	key := kind + ":" + external
	if _, ok := m[key]; !ok {
		m[key] = len(m) + 1
	}
	return m[key], nil
}

type school bool

func (s school) SchoolDay(time.Time) bool { return bool(s) }

func timetable(t *testing.T, pages []page, term school) *models.Timetable {
	t.Helper()
	data, err := writeZip(pages, map[string][]router.Point{}, term, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := gtfs.Built("test", func() ([]byte, error) { return data, nil }, 0, ids{}).Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Timetables) != 1 {
		t.Fatalf("%d timetables", len(snap.Timetables))
	}
	for _, tt := range snap.Timetables {
		return tt
	}
	return nil
}

func TestZip(t *testing.T) {
	out := route("110_M1", "110", "Gara (Penny)", "Ampoi 3", "MICESTI-Labului")
	back := route("110R_M2", "110R", "MICESTI-Labului", "Ampoi 3", "Gara CFR")
	for i, p := range []*page{&out, &back} {
		p.minutes = []int{0, 4, 9}
		for k := range p.stops {
			p.stops[k].at, p.stops[k].placed = router.Point{Lat: 46.07 + float64(i+k)/100, Lon: 23.57}, true
		}
	}
	out.departures = []departure{
		{day: calendar.Weekday, minute: 7 * 60}, {day: calendar.Weekday, minute: 7*60 + 30, only: term},
		{day: calendar.Weekday, minute: 8 * 60, only: holidays}, {day: calendar.Weekday, minute: 9 * 60, only: limit(time.Tuesday.String())},
		{day: calendar.Saturday, minute: 10 * 60},
	}
	back.departures = []departure{{day: calendar.Weekday, minute: 12 * 60}}
	pages := []page{out, back}
	column := func(s models.DaySchedule, in bool) []string {
		var times []string
		for _, e := range s.Entries {
			v := e.DepartureOut
			if in {
				v = e.DepartureIn
			}
			if v != "" {
				times = append(times, v)
			}
		}
		return times
	}
	inTerm := timetable(t, pages, true)
	if inTerm.RouteShortName != "110" || inTerm.InStopName != "Gara (Penny)" || inTerm.OutStopName != "MICESTI-Labului" {
		t.Fatalf("line %+v", inTerm)
	}
	if got := column(inTerm.Weekdays, true); !slices.Equal(got, []string{"07:00", "07:30"}) {
		t.Errorf("term weekdays %v", got)
	}
	if got := column(inTerm.Weekdays, false); !slices.Equal(got, []string{"12:00"}) {
		t.Errorf("back %v", got)
	}
	if got := column(inTerm.Saturday, true); !slices.Equal(got, []string{"10:00"}) {
		t.Errorf("saturday %v", got)
	}
	if got := column(timetable(t, pages, false).Weekdays, true); !slices.Equal(got, []string{"07:00", "08:00"}) {
		t.Errorf("holiday weekdays %v", got)
	}
}
