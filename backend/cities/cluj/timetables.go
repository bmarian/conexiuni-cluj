package cluj

import (
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/client"
	"golang.org/x/time/rate"
)

// CTP publishes its own timetables, more exact than Tranzy's: one CSV per line and kind of day.
type timetables struct {
	baseURL string
	http    *client.Client
	limiter *rate.Limiter
	cadence time.Duration
}

var errNoFile = errors.New("timetable not found")

func newTimetables(baseURL string, rateLimit, cadence time.Duration) *timetables {
	c := client.New()
	c.SetTimeout(30 * time.Second)
	return &timetables{baseURL: baseURL, http: c, limiter: rate.NewLimiter(rate.Every(rateLimit), 1), cadence: cadence}
}

func (s *timetables) Cadence() time.Duration { return s.cadence }

func (s *timetables) Timetables(snap *feeds.Snapshot) (map[string]*models.Timetable, error) {
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
			feeds.AlignDirections(t, feeds.FirstStopNames(snap, route.RouteID), "ctpcsv")
			mu.Lock()
			out[route.RouteShortName] = t
			mu.Unlock()
		}(route)
	}
	wg.Wait()
	return out, nil
}

// Days whose file is missing or unreadable stay empty.
func (s *timetables) timetable(line string) (*models.Timetable, error) {
	if err := s.limiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("ctpcj: rate limiter: %w", err)
	}
	empty := models.DaySchedule{Entries: []models.TimetableEntry{}}
	t := &models.Timetable{RouteShortName: line, Weekdays: empty, Saturday: empty, Sunday: empty}
	named := false
	for _, day := range []struct {
		file string
		into *models.DaySchedule
	}{{"lv", &t.Weekdays}, {"s", &t.Saturday}, {"d", &t.Sunday}} {
		parsed, err := s.day(line, day.file)
		if errors.Is(err, errNoFile) {
			continue
		}
		if err != nil {
			log.Printf("ctpcj: skip %s/%s: %v", line, day.file, err)
			continue
		}
		if !named {
			t.RouteLongName, t.InStopName, t.OutStopName, named = parsed.routeLongName, parsed.inStopName, parsed.outStopName, true
		}
		*day.into = parsed.DaySchedule
	}
	return t, nil
}

func (s *timetables) day(line, file string) (*csvDay, error) {
	resp, err := s.http.Get(fmt.Sprintf("%s/orar_%s_%s.csv", s.baseURL, line, file))
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
	case 404:
		return nil, errNoFile
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode())
	}
	parsed, err := parseCSV(resp.Body())
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return parsed, nil
}
