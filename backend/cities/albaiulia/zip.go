package albaiulia

import (
	"archive/zip"
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/feeds/gtfs"
	"conexiuni-cluj/router"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"
)

// The pages as a GTFS zip, its marked departures running on the days the school calendar gives them in the weeks the reader samples.
func writeZip(pages []page, shapes map[string][]router.Point, school feeds.School, now time.Time) ([]byte, error) {
	tables := map[string][][]string{
		"routes":         {{"route_id", "route_short_name", "route_type"}},
		"stops":          {{"stop_id", "stop_name", "stop_lat", "stop_lon"}},
		"trips":          {{"route_id", "service_id", "trip_id", "direction_id", "trip_headsign", "shape_id"}},
		"stop_times":     {{"trip_id", "arrival_time", "departure_time", "stop_id", "stop_sequence"}},
		"calendar":       {{"service_id", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "start_date", "end_date"}},
		"calendar_dates": {{"service_id", "date", "exception_type"}},
		"shapes":         {{"shape_id", "shape_pt_lat", "shape_pt_lon", "shape_pt_sequence"}},
	}
	add := func(table string, row ...string) { tables[table] = append(tables[table], row) }
	routes, stops, services := map[string]bool{}, map[string]bool{}, map[string]bool{}
	first := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 0, gtfs.SampledWeeks*7-1)
	service := func(day string, only limit) string {
		id := day
		if only != "" {
			id += "-" + string(only)
		}
		if services[id] {
			return id
		}
		services[id] = true
		runs := map[time.Weekday]bool{}
		for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
			if gtfs.DayKind(d) != day {
				continue
			}
			if only == "" {
				runs[d.Weekday()] = true
			} else if only.allows(d, school) {
				add("calendar_dates", id, d.Format("20060102"), "1")
			}
		}
		row := []string{id}
		for _, wd := range []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday} {
			row = append(row, flag(runs[wd]))
		}
		add("calendar", append(row, first.Format("20060102"), last.Format("20060102"))...)
		return id
	}
	for _, l := range lines(pages) {
		if !routes[l.name] {
			routes[l.name] = true
			add("routes", l.name, l.name, "3")
		}
		for dir, p := range l.ways {
			if p == nil {
				continue
			}
			var placed []int
			for i, s := range p.stops {
				if !s.placed {
					continue
				}
				placed = append(placed, i)
				if !stops[s.id] {
					stops[s.id] = true
					add("stops", s.id, stationName(s.name), coordinate(s.at.Lat), coordinate(s.at.Lon))
				}
			}
			if len(placed) < 2 {
				continue
			}
			for k, pt := range shapes[p.code] {
				add("shapes", p.code, coordinate(pt.Lat), coordinate(pt.Lon), strconv.Itoa(k))
			}
			headsign := stationName(p.stops[placed[len(placed)-1]].name)
			for n, d := range p.departures {
				trip := p.code + "-" + strconv.Itoa(n)
				add("trips", l.name, service(d.day, d.only), trip, strconv.Itoa(dir), headsign, p.code)
				for seq, i := range placed {
					at := clock(d.minute + p.minutes[i])
					add("stop_times", trip, at, at, p.stops[i].id, strconv.Itoa(seq))
				}
			}
		}
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, rows := range tables {
		f, err := w.Create(name + ".txt")
		if err != nil {
			return nil, err
		}
		if err := csv.NewWriter(f).WriteAll(rows); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func flag(on bool) string {
	if on {
		return "1"
	}
	return "0"
}

func coordinate(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func clock(minutes int) string { return fmt.Sprintf("%02d:%02d:00", minutes/60, minutes%60) }
