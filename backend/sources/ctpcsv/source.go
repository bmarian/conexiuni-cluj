package ctpcsv

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
	"log"
	"sync"
	"time"
)

// CTP Cluj's per-line CSV timetables, laid over a Snapshot whose trips carry no times.
type Source struct {
	client  *Client
	cadence time.Duration
}

func New(baseURL string, rateLimit, cadence time.Duration) *Source {
	return &Source{client: NewClient(baseURL, rateLimit), cadence: cadence}
}

func (s *Source) Cadence() time.Duration { return s.cadence }

func (s *Source) Timetables(snap *sources.Snapshot) (map[string]*models.Timetable, error) {
	out := make(map[string]*models.Timetable, len(snap.Routes))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, route := range snap.Routes {
		wg.Add(1)
		go func(route models.Route) {
			defer wg.Done()
			t, err := s.timetable(route.RouteShortName)
			if err != nil {
				log.Printf("ctpcsv: %s: %v", route.RouteShortName, err)
				return
			}
			alignDirections(t, firstStopNames(snap, route.RouteID))
			mu.Lock()
			out[route.RouteShortName] = t
			mu.Unlock()
		}(route)
	}
	wg.Wait()
	return out, nil
}

func (s *Source) timetable(routeShortName string) (*models.Timetable, error) {
	weekdays, saturday, sunday, err := s.client.FetchTimetable(routeShortName)
	if err != nil {
		return nil, err
	}
	t := &models.Timetable{
		RouteShortName: routeShortName,
		Weekdays:       models.DaySchedule{Entries: []models.TimetableEntry{}},
		Saturday:       models.DaySchedule{Entries: []models.TimetableEntry{}},
		Sunday:         models.DaySchedule{Entries: []models.TimetableEntry{}},
	}
	for _, parsed := range []*ParsedTimetable{weekdays, saturday, sunday} {
		if parsed != nil {
			t.RouteLongName, t.InStopName, t.OutStopName = parsed.RouteLongName, parsed.InStopName, parsed.OutStopName
			break
		}
	}
	if weekdays != nil {
		t.Weekdays = toDaySchedule(weekdays)
	}
	if saturday != nil {
		t.Saturday = toDaySchedule(saturday)
	}
	if sunday != nil {
		t.Sunday = toDaySchedule(sunday)
	}
	return t, nil
}

// The first stop of each direction, by direction id.
func firstStopNames(snap *sources.Snapshot, routeID int) [2]string {
	var first [2]models.TripStop
	var found [2]bool
	patterns := map[string]int{}
	for _, t := range snap.Trips {
		if t.RouteID == routeID {
			patterns[t.TripID] = int(t.DirectionID)
		}
	}
	for _, ts := range snap.TripStops {
		dir, ok := patterns[ts.TripID]
		if !ok || dir < 0 || dir > 1 {
			continue
		}
		if !found[dir] || ts.StopSequence < first[dir].StopSequence {
			first[dir], found[dir] = ts, true
		}
	}
	names := map[int]string{}
	for _, s := range snap.Stops {
		names[s.StopID] = s.StopName
	}
	return [2]string{names[first[0].StopID], names[first[1].StopID]}
}

// CTP's in/out columns don't follow the direction ids, so swap them when the stop names match the other way.
func alignDirections(t *models.Timetable, first [2]string) {
	if (t.InStopName == "" && t.OutStopName == "") || first[0] == "" || first[1] == "" {
		return
	}
	keep := stopMatchScore(t.InStopName, first[0]) + stopMatchScore(t.OutStopName, first[1])
	swap := stopMatchScore(t.InStopName, first[1]) + stopMatchScore(t.OutStopName, first[0])
	if swap <= keep {
		return
	}
	log.Printf("ctpcsv: %s: swapping directions (in=%q out=%q, first stops %q / %q)", t.RouteShortName, t.InStopName, t.OutStopName, first[0], first[1])
	t.InStopName, t.OutStopName = t.OutStopName, t.InStopName
	for _, d := range []*models.DaySchedule{&t.Weekdays, &t.Saturday, &t.Sunday} {
		for i := range d.Entries {
			d.Entries[i].DepartureIn, d.Entries[i].DepartureOut = d.Entries[i].DepartureOut, d.Entries[i].DepartureIn
		}
		d.InFrequency, d.OutFrequency = d.OutFrequency, d.InFrequency
	}
}

func toDaySchedule(p *ParsedTimetable) models.DaySchedule {
	entries := make([]models.TimetableEntry, len(p.Entries))
	for i, e := range p.Entries {
		entries[i] = models.TimetableEntry{DepartureIn: e.DepartureIn, DepartureOut: e.DepartureOut}
	}
	return models.DaySchedule{
		ServiceName:  p.ServiceName,
		ServiceStart: p.ServiceStart,
		Entries:      entries,
		InFrequency:  toFrequency(p.InFrequency),
		OutFrequency: toFrequency(p.OutFrequency),
	}
}

func toFrequency(f *Frequency) *models.Frequency {
	if f == nil {
		return nil
	}
	return &models.Frequency{Start: f.Start, End: f.End, MinMinutes: f.MinMinutes, MaxMinutes: f.MaxMinutes}
}
