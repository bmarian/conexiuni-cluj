// Package gtfs reads a static GTFS zip. For now it lays departure times over a Snapshot that has none.
package gtfs

import (
	"archive/zip"
	"bytes"
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Trips at or past this hour belong to the next service day, which lists them itself.
const serviceDayEndHour = 28

type Feed struct {
	url     string
	cadence time.Duration
}

func New(url string, cadence time.Duration) *Feed {
	return &Feed{url: url, cadence: cadence}
}

func (f *Feed) Cadence() time.Duration { return f.cadence }

type table struct {
	header map[string]int
	rows   [][]string
}

func (t table) get(row []string, column string) string {
	i, ok := t.header[column]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

type feed struct {
	files map[string]table
}

func (f *Feed) fetch() (*feed, error) {
	resp, err := http.Get(f.url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gtfs: %s: status %d", f.url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("gtfs: %s: %w", f.url, err)
	}
	out := &feed{files: map[string]table{}}
	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, ".txt")
		switch name {
		case "routes", "trips", "stops", "stop_times", "calendar", "calendar_dates":
		default:
			continue
		}
		r, err := file.Open()
		if err != nil {
			return nil, err
		}
		t, err := readTable(r)
		_ = r.Close()
		if err != nil {
			return nil, fmt.Errorf("gtfs: %s: %w", file.Name, err)
		}
		out.files[name] = t
	}
	for _, name := range []string{"routes", "trips", "stops", "stop_times", "calendar"} {
		if _, ok := out.files[name]; !ok {
			return nil, fmt.Errorf("gtfs: %s has no %s.txt", f.url, name)
		}
	}
	return out, nil
}

func readTable(r io.Reader) (table, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		return table{}, err
	}
	if len(rows) == 0 {
		return table{}, fmt.Errorf("empty")
	}
	header := map[string]int{}
	for i, name := range rows[0] {
		header[strings.TrimPrefix(strings.TrimSpace(name), "\ufeff")] = i
	}
	return table{header: header, rows: rows[1:]}, nil
}

// The services running on a date, from calendar.txt and the exceptions in calendar_dates.txt.
func (f *feed) servicesOn(day time.Time) map[string]bool {
	date := day.Format("20060102")
	weekday := strings.ToLower(day.Weekday().String())
	active := map[string]bool{}
	cal := f.files["calendar"]
	for _, row := range cal.rows {
		if cal.get(row, weekday) != "1" || date < cal.get(row, "start_date") || date > cal.get(row, "end_date") {
			continue
		}
		active[cal.get(row, "service_id")] = true
	}
	if dates, ok := f.files["calendar_dates"]; ok {
		for _, row := range dates.rows {
			if dates.get(row, "date") != date {
				continue
			}
			switch dates.get(row, "exception_type") {
			case "1":
				active[dates.get(row, "service_id")] = true
			case "2":
				delete(active, dates.get(row, "service_id"))
			}
		}
	}
	return active
}

// The dates the feed is valid for, from calendar.txt.
func (f *feed) validity() (start, end time.Time) {
	cal := f.files["calendar"]
	for _, row := range cal.rows {
		s, errS := time.Parse("20060102", cal.get(row, "start_date"))
		e, errE := time.Parse("20060102", cal.get(row, "end_date"))
		if errS != nil || errE != nil {
			continue
		}
		if start.IsZero() || s.Before(start) {
			start = s
		}
		if e.After(end) {
			end = e
		}
	}
	return start, end
}

// The first date of each kind of day the feed covers, from today or from the day the feed starts.
func representativeDays(now, start, end time.Time) map[string]time.Time {
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if start.After(from) {
		from = start
	}
	out := map[string]time.Time{}
	for i := 0; i < 7 && len(out) < 3; i++ {
		day := from.AddDate(0, 0, i)
		if !end.IsZero() && day.After(end) {
			break
		}
		kind := "weekdays"
		switch day.Weekday() {
		case time.Saturday:
			kind = "saturday"
		case time.Sunday:
			kind = "sunday"
		}
		if _, ok := out[kind]; !ok {
			out[kind] = day
		}
	}
	return out
}

type firstStop struct {
	stopID    string
	departure string
}

// Minutes of a GTFS time, which may pass 24:00; false when it belongs to the next service day.
func minutesOf(hhmmss string) (int, bool) {
	parts := strings.Split(hhmmss, ":")
	if len(parts) < 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h >= serviceDayEndHour {
		return 0, false
	}
	return h*60 + m, true
}

func (f *Feed) Timetables(snap *sources.Snapshot) (map[string]*models.Timetable, error) {
	data, err := f.fetch()
	if err != nil {
		return nil, err
	}
	routes := data.files["routes"]
	routeShort := map[string]string{}
	for _, row := range routes.rows {
		routeShort[routes.get(row, "route_id")] = routes.get(row, "route_short_name")
	}
	stops := data.files["stops"]
	stopName := map[string]string{}
	for _, row := range stops.rows {
		stopName[stops.get(row, "stop_id")] = stops.get(row, "stop_name")
	}

	type tripInfo struct {
		short, service string
		direction      int
	}
	trips := data.files["trips"]
	tripOf := map[string]tripInfo{}
	for _, row := range trips.rows {
		direction, _ := strconv.Atoi(trips.get(row, "direction_id"))
		tripOf[trips.get(row, "trip_id")] = tripInfo{short: routeShort[trips.get(row, "route_id")], service: trips.get(row, "service_id"), direction: direction}
	}

	// The first stop of every trip.
	stopTimes := data.files["stop_times"]
	firstOf := map[string]firstStop{}
	firstSeq := map[string]int{}
	for _, row := range stopTimes.rows {
		tripID := stopTimes.get(row, "trip_id")
		seq, _ := strconv.Atoi(stopTimes.get(row, "stop_sequence"))
		if prev, ok := firstSeq[tripID]; ok && seq >= prev {
			continue
		}
		firstSeq[tripID] = seq
		firstOf[tripID] = firstStop{stopID: stopTimes.get(row, "stop_id"), departure: stopTimes.get(row, "departure_time")}
	}

	start, end := data.validity()
	days := representativeDays(time.Now(), start, end)
	services := map[string]map[string]bool{}
	for kind, day := range days {
		services[kind] = data.servicesOn(day)
	}

	// Departures per line, kind of day and direction, and each direction's first stop.
	type key struct {
		short, kind string
		direction   int
	}
	departures := map[key]map[int]string{}
	termini := map[string][2]string{}
	for tripID, info := range tripOf {
		first, ok := firstOf[tripID]
		if !ok || info.short == "" || info.direction < 0 || info.direction > 1 {
			continue
		}
		if name := stopName[first.stopID]; name != "" {
			t := termini[info.short]
			if t[info.direction] == "" {
				t[info.direction] = name
				termini[info.short] = t
			}
		}
		minutes, ok := minutesOf(first.departure)
		if !ok {
			continue
		}
		for kind, active := range services {
			if !active[info.service] {
				continue
			}
			k := key{info.short, kind, info.direction}
			if departures[k] == nil {
				departures[k] = map[int]string{}
			}
			departures[k][minutes] = fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
		}
	}

	out := make(map[string]*models.Timetable, len(snap.Routes))
	for _, route := range snap.Routes {
		short := route.RouteShortName
		if _, ok := termini[short]; !ok {
			continue
		}
		t := &models.Timetable{
			RouteShortName: short,
			RouteLongName:  route.RouteLongName,
			InStopName:     termini[short][0],
			OutStopName:    termini[short][1],
		}
		for kind, schedule := range map[string]*models.DaySchedule{"weekdays": &t.Weekdays, "saturday": &t.Saturday, "sunday": &t.Sunday} {
			*schedule = daySchedule(sortedTimes(departures[key{short, kind, 0}]), sortedTimes(departures[key{short, kind, 1}]))
		}
		sources.AlignDirections(t, sources.FirstStopNames(snap, route.RouteID))
		out[short] = t
	}
	return out, nil
}

func sortedTimes(byMinute map[int]string) []string {
	minutes := make([]int, 0, len(byMinute))
	for m := range byMinute {
		minutes = append(minutes, m)
	}
	sort.Ints(minutes)
	out := make([]string, len(minutes))
	for i, m := range minutes {
		out[i] = byMinute[m]
	}
	return out
}

// Two columns of departures, row by row, like the printed timetables the app grew up with.
func daySchedule(in, out []string) models.DaySchedule {
	n := max(len(in), len(out))
	entries := make([]models.TimetableEntry, n)
	for i := range entries {
		if i < len(in) {
			entries[i].DepartureIn = in[i]
		}
		if i < len(out) {
			entries[i].DepartureOut = out[i]
		}
	}
	return models.DaySchedule{Entries: entries}
}
