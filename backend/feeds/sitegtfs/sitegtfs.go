// Package sitegtfs writes a network read from an operator's own pages as a GTFS zip, for gtfs.Built to read like any other.
package sitegtfs

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

// One way of a route as the site gives it.
type Route struct {
	// Unique across the site; also the shape's id.
	ID    string
	Stops []Stop
	// Minutes from the first stop to each; nil when the site times only the first stop.
	Minutes    []int
	Departures []Departure
}

type Stop struct {
	// Unique across the site.
	ID     string
	Name   string
	At     router.Point
	Placed bool
}

type Departure struct {
	// A kind of day from the calendar package.
	Day    string
	Minute int
	Only   Limit
}

// A line of the app: up to two ways under one name; several lines may share a name, and the GTFS reader then keeps the busiest route each way.
type Line struct {
	Name string
	Ways [2]*Route
}

// Zip writes the lines, their marked departures running on the days the school calendar gives them in the weeks the reader samples.
func Zip(lines []Line, shapes map[string][]router.Point, school feeds.School, now time.Time) ([]byte, error) {
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
	service := func(day string, only Limit) string {
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
			} else if only.Allows(d, school) {
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
	for _, l := range lines {
		if !routes[l.Name] {
			routes[l.Name] = true
			add("routes", l.Name, l.Name, "3")
		}
		for dir, r := range l.Ways {
			if r == nil {
				continue
			}
			var placed []int
			for i, s := range r.Stops {
				if !s.Placed {
					continue
				}
				placed = append(placed, i)
				if !stops[s.ID] {
					stops[s.ID] = true
					add("stops", s.ID, s.Name, coordinate(s.At.Lat), coordinate(s.At.Lon))
				}
			}
			if len(placed) < 2 {
				continue
			}
			for k, pt := range shapes[r.ID] {
				add("shapes", r.ID, coordinate(pt.Lat), coordinate(pt.Lon), strconv.Itoa(k))
			}
			headsign := r.Stops[placed[len(placed)-1]].Name
			for n, d := range r.Departures {
				trip := r.ID + "-" + strconv.Itoa(n)
				add("trips", l.Name, service(d.Day, d.Only), trip, strconv.Itoa(dir), headsign, r.ID)
				for seq, i := range placed {
					at := ""
					if seq == 0 || r.Minutes != nil {
						at = clock(d.Minute + minutes(r, i))
					}
					add("stop_times", trip, at, at, r.Stops[i].ID, strconv.Itoa(seq))
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

func minutes(r *Route, i int) int {
	if r.Minutes == nil {
		return 0
	}
	return r.Minutes[i]
}

func flag(on bool) string {
	if on {
		return "1"
	}
	return "0"
}

func coordinate(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func clock(minutes int) string { return fmt.Sprintf("%02d:%02d:00", minutes/60, minutes%60) }
