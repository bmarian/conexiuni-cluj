// Package gtfs reads a static GTFS zip, as a whole network or as times for another source's Snapshot.
package gtfs

import (
	"archive/zip"
	"bytes"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Later trips belong to the next service day, which lists them itself.
const serviceDayEndHour = 28

// Weeks sampled ahead: enough to outvote a holiday.
const representativeWeeks = 4

type Feed struct {
	url     string
	cadence time.Duration
	ids     feeds.IDMap
}

// ids may be nil when the feed only adds times to another source's Snapshot.
func New(url string, cadence time.Duration, ids feeds.IDMap) *Feed {
	return &Feed{url: url, cadence: cadence, ids: ids}
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
	files      map[string]table
	trips      map[string]tripInfo
	stops      tripStops
	patterns   map[string]*pattern
	stopName   map[string]string
	exceptions map[string][]exception
	headways   map[string]headway
	dropped    int
	inferred   int
	url        string
}

func (f *Feed) fetch() (*feed, error) {
	data, err := feeds.Download(f.url)
	if err != nil {
		return nil, fmt.Errorf("gtfs: %w", err)
	}
	out, err := parse(data)
	if err != nil {
		return nil, err
	}
	out.url = f.url
	if out.dropped > 0 {
		log.Printf("gtfs %s: %d trips run only part of their line and are not listed", f.url, out.dropped)
	}
	if out.inferred > 0 {
		log.Printf("gtfs %s: %d lines name no direction, so it is taken from where their runs head", f.url, out.inferred)
	}
	return out, nil
}

func parse(data []byte) (*feed, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("gtfs: %w", err)
	}
	out := &feed{files: map[string]table{}}
	for _, file := range archive.File {
		name := strings.TrimSuffix(file.Name, ".txt")
		switch name {
		case "routes", "trips", "stops", "stop_times", "calendar", "calendar_dates", "shapes", "frequencies":
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
			return nil, fmt.Errorf("gtfs: no %s.txt", name)
		}
	}
	stops := out.files["stops"]
	out.stopName = make(map[string]string, len(stops.rows))
	at := make(map[string]point, len(stops.rows))
	for _, row := range stops.rows {
		id := stops.get(row, "stop_id")
		out.stopName[id] = stops.get(row, "stop_name")
		lat, errLat := strconv.ParseFloat(stops.get(row, "stop_lat"), 64)
		lon, errLon := strconv.ParseFloat(stops.get(row, "stop_lon"), 64)
		if errLat == nil && errLon == nil {
			at[id] = point{lat, lon}
		}
	}
	out.trips, out.stops, out.exceptions = out.readTrips(), out.readStopTimes(), out.readExceptions()
	out.headways = out.expandFrequencies()
	out.inferred = inferDirections(out.trips, out.stops, at)
	out.patterns, out.dropped = patterns(out.trips, out.stops)
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

type tripInfo struct {
	routeID, service, headsign, shapeID string
	direction                           int
}

type stopTime struct {
	stopID    string
	seq       int
	arrival   int // minutes, -1 when missing
	departure int
}

type tripStops map[string][]stopTime

func (f *feed) readTrips() map[string]tripInfo {
	trips := f.files["trips"]
	out := make(map[string]tripInfo, len(trips.rows))
	for _, row := range trips.rows {
		direction := unknownDirection
		if raw := strings.TrimSpace(trips.get(row, "direction_id")); raw != "" {
			direction, _ = strconv.Atoi(raw)
		}
		out[trips.get(row, "trip_id")] = tripInfo{
			routeID: trips.get(row, "route_id"), service: trips.get(row, "service_id"),
			headsign: trips.get(row, "trip_headsign"), shapeID: trips.get(row, "shape_id"), direction: direction,
		}
	}
	return out
}

func (f *feed) readStopTimes() tripStops {
	st := f.files["stop_times"]
	out := tripStops{}
	for _, row := range st.rows {
		tripID := st.get(row, "trip_id")
		seq, _ := strconv.Atoi(st.get(row, "stop_sequence"))
		out[tripID] = append(out[tripID], stopTime{
			stopID: st.get(row, "stop_id"), seq: seq,
			arrival: minutesOrMissing(st.get(row, "arrival_time")), departure: minutesOrMissing(st.get(row, "departure_time")),
		})
	}
	for _, stops := range out {
		sort.Slice(stops, func(i, j int) bool { return stops[i].seq < stops[j].seq })
	}
	return out
}

// Minutes of a GTFS time, which may pass 24:00; -1 when missing.
func minutesOrMissing(hhmmss string) int {
	parts := strings.Split(hhmmss, ":")
	if len(parts) < 2 {
		return -1
	}
	h, errH := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil {
		return -1
	}
	return h*60 + m
}

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
	for _, e := range f.exceptions[date] {
		if e.added {
			active[e.service] = true
		} else {
			delete(active, e.service)
		}
	}
	return active
}

type exception struct {
	service string
	added   bool
}

func (f *feed) readExceptions() map[string][]exception {
	out := map[string][]exception{}
	dates, ok := f.files["calendar_dates"]
	if !ok {
		return out
	}
	for _, row := range dates.rows {
		switch kind := dates.get(row, "exception_type"); kind {
		case "1", "2":
			date := dates.get(row, "date")
			out[date] = append(out[date], exception{service: dates.get(row, "service_id"), added: kind == "1"})
		}
	}
	return out
}

func (f *feed) validity() (start, end time.Time) {
	widen := func(s, e string) {
		from, errS := time.Parse("20060102", s)
		to, errE := time.Parse("20060102", e)
		if errS != nil || errE != nil {
			return
		}
		if start.IsZero() || from.Before(start) {
			start = from
		}
		if to.After(end) {
			end = to
		}
	}
	cal := f.files["calendar"]
	for _, row := range cal.rows {
		widen(cal.get(row, "start_date"), cal.get(row, "end_date"))
	}
	for date, list := range f.exceptions {
		for _, e := range list {
			if e.added {
				widen(date, date)
				break
			}
		}
	}
	return start, end
}

func dayKind(day time.Time) string {
	switch day.Weekday() {
	case time.Saturday:
		return calendar.Saturday
	case time.Sunday:
		return calendar.Sunday
	}
	return calendar.Weekday
}

// The first date of each kind running what most days of that kind run, so a one-off day never stands for it.
func (f *feed) representativeDays(now time.Time) map[string]time.Time {
	start, end := f.validity()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if start.After(from) {
		from = start
	}
	if end.Before(from) {
		end = from.AddDate(0, 0, 6)
	}
	if horizon := from.AddDate(0, 0, representativeWeeks*7-1); end.After(horizon) {
		end = horizon
	}
	type tally struct {
		first time.Time
		days  int
	}
	byKind := map[string]map[string]*tally{}
	for day := from; !day.After(end); day = day.AddDate(0, 0, 1) {
		active := f.servicesOn(day)
		services := make([]string, 0, len(active))
		for id := range active {
			services = append(services, id)
		}
		sort.Strings(services)
		kind, sig := dayKind(day), strings.Join(services, "\x00")
		if byKind[kind] == nil {
			byKind[kind] = map[string]*tally{}
		}
		if t := byKind[kind][sig]; t != nil {
			t.days++
		} else {
			byKind[kind][sig] = &tally{first: day, days: 1}
		}
	}
	out := map[string]time.Time{}
	for kind, tallies := range byKind {
		var best *tally
		for _, t := range tallies {
			if best == nil || t.days > best.days || (t.days == best.days && t.first.Before(best.first)) {
				best = t
			}
		}
		out[kind] = best.first
	}
	return out
}

func (f *feed) servicesByKind(now time.Time) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for kind, day := range f.representativeDays(now) {
		out[kind] = f.servicesOn(day)
	}
	return out
}

func startMinutes(run []stopTime) int {
	if run[0].departure >= 0 {
		return run[0].departure
	}
	return run[0].arrival
}

// Matched to the Snapshot's routes by line number.
func (f *Feed) Timetables(snap *feeds.Snapshot) (map[string]*models.Timetable, error) {
	data, err := f.fetch()
	if err != nil {
		return nil, err
	}
	return data.timetables(snap, time.Now()), nil
}

type lineDir struct {
	line      string
	direction int
}

func (f *feed) timetables(snap *feeds.Snapshot, now time.Time) map[string]*models.Timetable {
	routes := f.files["routes"]
	routeShort := map[string]string{}
	for _, row := range routes.rows {
		routeShort[routes.get(row, "route_id")] = routes.get(row, "route_short_name")
	}
	services := f.servicesByKind(now)

	// First-stop departures per line, direction and kind of day.
	type key struct {
		lineDir
		kind string
	}
	departures := map[key]map[int]string{}
	windows := map[key][]headway{}
	termini := map[lineDir]string{}
	for _, p := range f.patterns {
		k := lineDir{routeShort[p.routeID], p.dir}
		if k.line == "" {
			continue
		}
		termini[k] = p.stops[0]
		for _, tripID := range p.trips {
			minutes := startMinutes(f.stops[tripID])
			if minutes < 0 || minutes >= serviceDayEndHour*60 {
				continue
			}
			for kind, active := range services {
				if !active[f.trips[tripID].service] {
					continue
				}
				dk := key{k, kind}
				if departures[dk] == nil {
					departures[dk] = map[int]string{}
				}
				departures[dk][minutes] = fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
				if w, ok := f.headways[tripID]; ok && !slices.Contains(windows[dk], w) {
					windows[dk] = append(windows[dk], w)
				}
			}
		}
	}

	out := make(map[string]*models.Timetable, len(snap.Routes))
	for _, route := range snap.Routes {
		short := route.RouteShortName
		in, back := termini[lineDir{short, 0}], termini[lineDir{short, 1}]
		if in == "" && back == "" {
			continue
		}
		t := &models.Timetable{
			RouteShortName: short,
			RouteLongName:  route.RouteLongName,
			InStopName:     f.stopName[in],
			OutStopName:    f.stopName[back],
		}
		for kind, schedule := range map[string]*models.DaySchedule{calendar.Weekday: &t.Weekdays, calendar.Saturday: &t.Saturday, calendar.Sunday: &t.Sunday} {
			k0, k1 := key{lineDir{short, 0}, kind}, key{lineDir{short, 1}, kind}
			*schedule = feeds.Columns(sortedTimes(departures[k0]), sortedTimes(departures[k1]))
			schedule.InFrequency, schedule.OutFrequency = frequencyOf(windows[k0]), frequencyOf(windows[k1])
		}
		feeds.AlignDirections(t, feeds.FirstStopNames(snap, route.RouteID), "gtfs "+f.url)
		out[short] = t
	}
	return out
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

func (f *Feed) Snapshot() (*feeds.Snapshot, error) {
	if f.ids == nil {
		return nil, fmt.Errorf("gtfs: %s has no id map, so it can only lay times over another source", f.url)
	}
	data, err := f.fetch()
	if err != nil {
		return nil, err
	}
	return data.snapshot(f.ids, time.Now())
}

type pattern struct {
	routeID  string
	dir      int
	stops    []string
	trips    []string // the trips following it exactly, then those folded in
	headsign string
	shapeID  string
	// Runs that only add stops, folded in when their timing agrees.
	candidates []string
}

// The app shows whole minutes, so runs less than one apart are the same bus to a rider.
const foldToleranceSec = 60

// The stop sequence most trips follow, per route and direction. Short runs and other paths are left out and counted.
func patterns(trips map[string]tripInfo, stops tripStops) (map[string]*pattern, int) {
	type group struct {
		bySignature map[string][]string
		order       map[string][]string
	}
	groups := map[lineDir]*group{}
	for tripID, info := range trips {
		st := stops[tripID]
		if len(st) < 2 || info.direction < 0 || info.direction > 1 {
			continue
		}
		seq := make([]string, len(st))
		for i, s := range st {
			seq[i] = s.stopID
		}
		k := lineDir{info.routeID, info.direction}
		g := groups[k]
		if g == nil {
			g = &group{bySignature: map[string][]string{}, order: map[string][]string{}}
			groups[k] = g
		}
		sig := strings.Join(seq, "\x00")
		g.bySignature[sig] = append(g.bySignature[sig], tripID)
		g.order[sig] = seq
	}
	out := map[string]*pattern{}
	dropped := 0
	for k, g := range groups {
		best, n := "", -1
		for sig, ts := range g.bySignature {
			if len(ts) > n || (len(ts) == n && sig < best) {
				best, n = sig, len(ts)
			}
		}
		id := k.line + "_" + strconv.Itoa(k.direction)
		p := &pattern{routeID: k.line, dir: k.direction, stops: g.order[best], trips: g.bySignature[best]}
		sort.Strings(p.trips)
		first := trips[p.trips[0]]
		p.headsign, p.shapeID = first.headsign, first.shapeID
		out[id] = p
		for sig, ts := range g.bySignature {
			switch {
			case sig == best:
			case passesAll(p.stops, g.order[sig]):
				p.candidates = append(p.candidates, ts...)
			default:
				dropped += len(ts)
			}
		}
		sort.Strings(p.candidates)
		offsets := scheduledOffsets(p, stops)
		for _, tripID := range p.candidates {
			if deviationSec(p.stops, offsets, stops[tripID]) < foldToleranceSec {
				p.trips = append(p.trips, tripID)
			} else {
				dropped++
			}
		}
	}
	return out, dropped
}

// The run's worst timing gap to the pattern, at the pattern's stops.
func deviationSec(patternStops []string, offsets []float64, run []stopTime) float64 {
	start := startMinutes(run)
	worst, cum, i := 0.0, 0.0, 0
	for _, st := range run {
		if i >= len(patternStops) || st.stopID != patternStops[i] {
			continue
		}
		cum += offsets[i]
		at := st.arrival
		if at < 0 {
			at = st.departure
		}
		worst = max(worst, math.Abs(float64((at-start)*60)-cum))
		i++
	}
	return worst
}

func passesAll(pattern, run []string) bool {
	i := 0
	for _, stop := range run {
		if i < len(pattern) && stop == pattern[i] {
			i++
		}
	}
	return i == len(pattern)
}

func (f *feed) snapshot(ids feeds.IDMap, now time.Time) (*feeds.Snapshot, error) {
	routes := f.files["routes"]
	routeIDs := map[string]int{}
	snap := &feeds.Snapshot{Offsets: map[string][]float64{}, HourlyOffsets: map[string]map[string][][]float64{}}
	for _, row := range routes.rows {
		external := routes.get(row, "route_id")
		id, err := ids.ExternalID("route", external)
		if err != nil {
			return nil, err
		}
		routeIDs[external] = id
		routeType, _ := strconv.Atoi(routes.get(row, "route_type"))
		color := strings.TrimPrefix(routes.get(row, "route_color"), "#")
		if len(color) == 6 {
			color = "#" + strings.ToUpper(color)
		} else {
			color = models.ResolveRouteDisplayColor(routes.get(row, "route_short_name"))
		}
		longName := routes.get(row, "route_long_name")
		if longName == "" {
			longName = routeName(f.patterns, external, f.stopName)
		}
		snap.Routes = append(snap.Routes, models.Route{
			RouteID: id, RouteShortName: routes.get(row, "route_short_name"), RouteLongName: longName,
			RouteType: models.RouteType(routeType), RouteDesc: routes.get(row, "route_desc"), RouteColor: color,
		})
	}

	stopIDs := map[string]int{}
	stopByExternal := map[string]models.Stop{}
	used := map[string]bool{}
	for _, p := range f.patterns {
		for _, s := range p.stops {
			used[s] = true
		}
	}
	stopsTable := f.files["stops"]
	for _, row := range stopsTable.rows {
		external := stopsTable.get(row, "stop_id")
		if !used[external] {
			continue
		}
		id, err := ids.ExternalID("stop", external)
		if err != nil {
			return nil, err
		}
		lat, _ := strconv.ParseFloat(stopsTable.get(row, "stop_lat"), 64)
		lon, _ := strconv.ParseFloat(stopsTable.get(row, "stop_lon"), 64)
		locationType, _ := strconv.Atoi(stopsTable.get(row, "location_type"))
		stop := models.Stop{
			StopID: id, StopName: stopsTable.get(row, "stop_name"), StopDesc: stopsTable.get(row, "stop_desc"),
			StopLat: lat, StopLon: lon, LocationType: models.LocationType(locationType), StopCode: stopsTable.get(row, "stop_code"),
		}
		stopIDs[external] = id
		stopByExternal[external] = stop
		snap.Stops = append(snap.Stops, stop)
	}

	shapePoints := map[string][]models.Shape{}
	if shapes, ok := f.files["shapes"]; ok {
		for _, row := range shapes.rows {
			id := shapes.get(row, "shape_id")
			lat, _ := strconv.ParseFloat(shapes.get(row, "shape_pt_lat"), 64)
			lon, _ := strconv.ParseFloat(shapes.get(row, "shape_pt_lon"), 64)
			seq, _ := strconv.Atoi(shapes.get(row, "shape_pt_sequence"))
			dist, _ := strconv.ParseFloat(shapes.get(row, "shape_dist_traveled"), 64)
			shapePoints[id] = append(shapePoints[id], models.Shape{ShapeID: id, ShapePtLat: lat, ShapePtLon: lon, ShapePtSequence: seq, ShapeDistTraveled: dist})
		}
	}

	services := f.servicesByKind(now)
	keys := make([]string, 0, len(f.patterns))
	for id := range f.patterns {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, key := range keys {
		p := f.patterns[key]
		routeID, ok := routeIDs[p.routeID]
		if !ok {
			continue
		}
		patternID := models.PatternID(routeID, p.dir)
		snap.Trips = append(snap.Trips, models.Trip{
			TripID: patternID, RouteID: routeID, DirectionID: models.DirectionType(p.dir), TripHeadsign: p.headsign, ShapeID: patternID,
		})
		for i, external := range p.stops {
			snap.TripStops = append(snap.TripStops, models.TripStop{TripID: patternID, StopID: stopIDs[external], StopSequence: i})
		}
		points := append([]models.Shape(nil), shapePoints[p.shapeID]...)
		if len(points) < 2 {
			// No shape: straight legs between the stops.
			points = points[:0]
			for i, external := range p.stops {
				s := stopByExternal[external]
				points = append(points, models.Shape{ShapePtLat: s.StopLat, ShapePtLon: s.StopLon, ShapePtSequence: i})
			}
		}
		sort.Slice(points, func(i, j int) bool { return points[i].ShapePtSequence < points[j].ShapePtSequence })
		for i := range points {
			points[i].ShapeID = patternID
			points[i].ShapePtSequence = i
			snap.Shapes = append(snap.Shapes, points[i])
		}
		snap.Offsets[patternID] = scheduledOffsets(p, f.stops)
		snap.HourlyOffsets[patternID] = f.hourlyOffsets(p, services)
	}

	snap.Timetables = f.timetables(snap, now)
	return snap, nil
}

// An unnamed route is called after its ends, as operators name theirs.
func routeName(pats map[string]*pattern, routeID string, stopName map[string]string) string {
	var ends, headsigns [2]string
	for _, p := range pats {
		if p.routeID != routeID || p.dir < 0 || p.dir > 1 || len(p.stops) == 0 {
			continue
		}
		ends[p.dir], headsigns[p.dir] = stopName[p.stops[0]], p.headsign
	}
	switch {
	case ends[0] != "" && ends[1] != "":
		return ends[0] + " - " + ends[1]
	case ends[0] != "" && headsigns[0] != "":
		return ends[0] + " - " + headsigns[0]
	case ends[1] != "" && headsigns[1] != "":
		return ends[1] + " - " + headsigns[1]
	}
	return ""
}

// Seconds between consecutive stops, the median over the trips that follow the pattern exactly.
func scheduledOffsets(p *pattern, stops tripStops) []float64 {
	n := len(p.stops)
	samples := make([][]float64, n)
	for _, tripID := range p.trips {
		st := stops[tripID]
		if len(st) != n {
			continue
		}
		for i := 1; i < n; i++ {
			from, to := st[i-1].departure, st[i].arrival
			if from < 0 {
				from = st[i-1].arrival
			}
			if to < 0 {
				to = st[i].departure
			}
			if from >= 0 && to >= from {
				samples[i] = append(samples[i], float64(to-from)*60)
			}
		}
	}
	out := make([]float64, n)
	for i := 1; i < n; i++ {
		if len(samples[i]) == 0 {
			continue
		}
		sort.Float64s(samples[i])
		out[i] = samples[i][len(samples[i])/2]
	}
	return out
}

// Hours no run leaves in take the nearest hour's, earlier first.
func (f *feed) hourlyOffsets(p *pattern, services map[string]map[string]bool) map[string][][]float64 {
	out := map[string][][]float64{}
	for kind, active := range services {
		var byHour [24][]string
		for _, tripID := range p.trips {
			st := f.stops[tripID]
			start := startMinutes(st)
			if len(st) != len(p.stops) || !active[f.trips[tripID].service] || start < 0 || start >= serviceDayEndHour*60 {
				continue
			}
			byHour[start/60%24] = append(byHour[start/60%24], tripID)
		}
		var own [24][]float64
		running := false
		for hour, trips := range byHour {
			if len(trips) > 0 {
				own[hour], running = rideMedians(trips, len(p.stops), f.stops), true
			}
		}
		if !running {
			continue
		}
		hours := make([][]float64, 24)
		for hour := range hours {
			for d := 0; hours[hour] == nil; d++ {
				if earlier := own[(hour-d+24)%24]; earlier != nil {
					hours[hour] = earlier
				} else {
					hours[hour] = own[(hour+d)%24]
				}
			}
		}
		out[kind] = hours
	}
	return out
}

// From the median time to reach each stop: per-segment medians add up to times no run keeps.
func rideMedians(trips []string, n int, stops tripStops) []float64 {
	samples := make([][]int, n)
	for _, tripID := range trips {
		st := stops[tripID]
		start := startMinutes(st)
		if len(st) != n || start < 0 {
			continue
		}
		for i := 1; i < n; i++ {
			at := st[i].arrival
			if at < 0 {
				at = st[i].departure
			}
			if at >= start {
				samples[i] = append(samples[i], at-start)
			}
		}
	}
	reached := make([]int, n)
	for i := 1; i < n; i++ {
		if len(samples[i]) == 0 {
			reached[i] = -1
			continue
		}
		sort.Ints(samples[i])
		reached[i] = samples[i][len(samples[i])/2]
	}
	out := make([]float64, n)
	for i := 1; i < n; i++ {
		if reached[i] < 0 || reached[i-1] < 0 {
			out[i] = -1
			continue
		}
		out[i] = float64(max(reached[i]-reached[i-1], 0) * 60)
	}
	return out
}
