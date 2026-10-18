package ctpcsv

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources"
	"log"
	"sync"
	"time"
)

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
			sources.AlignDirections(t, sources.FirstStopNames(snap, route.RouteID))
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
