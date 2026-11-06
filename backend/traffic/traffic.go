package traffic

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/live"
	"conexiuni-cluj/models"
	"conexiuni-cluj/transit"
	"conexiuni-cluj/web"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	HoursPerDay = 24
	LiveWindow  = 30 * time.Minute
	liveEvery   = time.Minute
	// Band 1 stays in the numbering older apps colour by.
	slowBand   = 2
	cutsLayout = "2006-01-02"
)

type Timing interface {
	Stretches(tripID string) ([]live.Stretch, bool)
	SegmentProfiles(routeID, directionID int) ([]live.SegmentProfile, error)
	EachStretchRun(since time.Time, fn func(live.StretchRun)) error
	Period() int64
}

// Still read by apps from before traffic came with the live stream.
type Now struct {
	Lines map[string][]models.TrafficStretch `json:"lines"`
}

type Service struct {
	loc     *time.Location
	store   *database.Store
	data    *transit.Data
	offered *transit.Availability
	timing  Timing
	// A timetable-only city has no bus timings to colour by.
	realtime bool
	Runner   func() *jobs.Runner

	mu       sync.Mutex
	period   int64
	building bool
	all      *measured
	bands    []int
	bandsDay string
	rated    *ratedNow
}

type ratedNow struct {
	lines map[string]models.Traffic
	at    time.Time
}

func New(loc *time.Location, store *database.Store, data *transit.Data, offered *transit.Availability, timing Timing, realtime bool) *Service {
	return &Service{loc: loc, store: store, data: data, offered: offered, timing: timing, realtime: realtime, Runner: jobs.None}
}

func (s *Service) Register(api fiber.Router) {
	api.Get("/traffic", s.serve)
}

func (s *Service) Warm() {
	if !s.realtime {
		return
	}
	s.mu.Lock()
	s.rebuild(time.Now().In(s.loc))
	s.mu.Unlock()
}

func (s *Service) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.all != nil && len(s.bands) > 0
}

func (s *Service) serve(c fiber.Ctx) error {
	now, err := s.now(time.Now().In(s.loc))
	if err != nil {
		c.Set("Cache-Control", "no-store")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	out := Now{Lines: make(map[string][]models.TrafficStretch)}
	for _, id := range strings.Split(c.Query("trip_ids"), ",") {
		if line, ok := now.lines[id]; ok {
			out.Lines[id] = line.Stretches
		}
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(out)
}

var (
	errBuilding  = errors.New("the traffic map is still being built")
	errNoTimings = errors.New("no live buses to measure traffic from")
)

func (s *Service) now(at time.Time) (*ratedNow, error) {
	if !s.realtime {
		return nil, errNoTimings
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.all == nil || s.timing.Period() != s.period || s.bandsDay != at.Format(cutsLayout) {
		s.rebuild(at)
	}
	if s.all == nil || len(s.bands) == 0 {
		return nil, errBuilding
	}
	if s.rated != nil && at.Sub(s.rated.at) < liveEvery {
		return s.rated, nil
	}
	runs, err := s.recent(at)
	if err != nil {
		return nil, err
	}
	s.rated = &ratedNow{lines: s.all.now(runs, s.bands), at: at}
	return s.rated, nil
}

func (s *Service) Lines() map[string]models.Traffic {
	now, err := s.now(time.Now().In(s.loc))
	if err != nil {
		return nil
	}
	return now.lines
}

func (s *Service) recent(at time.Time) (map[pair][]float64, error) {
	runs := make(map[pair][]float64)
	err := s.timing.EachStretchRun(at.Add(-LiveWindow), func(r live.StretchRun) {
		k := pair{r.FromStopID, r.ToStopID}
		runs[k] = append(runs[k], r.DurationSec)
	})
	if err != nil {
		return nil, fmt.Errorf("recent bus times: %w", err)
	}
	return runs, nil
}

// rebuild measures in the background, once at a time; the caller holds the lock.
func (s *Service) rebuild(at time.Time) {
	if s.building {
		return
	}
	s.building = true
	period, day := s.timing.Period(), at.Format(cutsLayout)
	learnBands := s.bandsDay != day
	started := s.Runner().Go(func() {
		all, err := s.measure()
		var bands []int
		if err == nil && learnBands {
			bands, err = s.learnBands(all)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.building = false
		if err != nil {
			s.store.Logf("traffic: build failed: %v", err)
			return
		}
		s.all, s.period, s.rated = all, period, nil
		if learnBands {
			s.bands, s.bandsDay = bands, day
		}
	})
	if !started {
		s.building = false
	}
}

type cell struct {
	dayType string
	hour    int
}

type pair struct {
	from, to int
}

type measuredStretch struct {
	pair pair
	path string
}

type measured struct {
	lines       map[string][]*measuredStretch
	free        map[pair]float64
	quiet, rush map[cell]bool
}

func (m *measured) now(runs map[pair][]float64, bands []int) map[string]models.Traffic {
	out := make(map[string]models.Traffic)
	for id, line := range m.lines {
		var t models.Traffic
		last := -1
		for i, st := range line {
			r, ok := m.reading(line, i, runs)
			b := band(percent(r), bands)
			if !ok || b < slowBand {
				continue
			}
			t.Stretches = append(t.Stretches, models.TrafficStretch{Path: st.path, Band: b})
			delay := int(math.Round((r - 1) * m.free[st.pair]))
			if jam := len(t.Jams) - 1; jam >= 0 && last == i-1 {
				t.Jams[jam].ToStopID, t.Jams[jam].Band, t.Jams[jam].DelaySec = st.pair.to, max(t.Jams[jam].Band, b), t.Jams[jam].DelaySec+delay
			} else {
				t.Jams = append(t.Jams, models.Jam{FromStopID: st.pair.from, ToStopID: st.pair.to, Band: b, DelaySec: delay})
			}
			last = i
		}
		if len(t.Stretches) > 0 {
			out[id] = t
		}
	}
	return out
}

// Pools the neighbours' buses to smooth out lights and stops.
func (m *measured) reading(line []*measuredStretch, i int, runs map[pair][]float64) (float64, bool) {
	if len(runs[line[i].pair]) == 0 || m.free[line[i].pair] <= 0 {
		return 0, false
	}
	var ratios []float64
	for j := max(i-1, 0); j <= min(i+1, len(line)-1); j++ {
		free := m.free[line[j].pair]
		if free <= 0 {
			continue
		}
		for _, sec := range runs[line[j].pair] {
			ratios = append(ratios, sec/free)
		}
	}
	return live.Median(ratios), true
}

func (s *Service) measure() (*measured, error) {
	started := time.Now()
	routes, err := s.data.Routes(transit.RouteFilter{})
	if err != nil {
		return nil, fmt.Errorf("routes: %w", err)
	}
	out := &measured{lines: make(map[string][]*measuredStretch)}
	pairs := make(map[pair]*secStats)
	for _, r := range routes {
		if s.data.LineHidden(r.RouteShortName) || (s.offered.IsReady() && !s.offered.RouteHasTimetable(r.RouteShortName)) {
			continue
		}
		for direction := range 2 {
			id := models.PatternID(r.RouteID, direction)
			stretches, ok := s.timing.Stretches(id)
			if !ok {
				continue
			}
			profiles, err := s.timing.SegmentProfiles(r.RouteID, direction)
			if err != nil {
				return nil, fmt.Errorf("profiles of %s: %w", r.RouteShortName, err)
			}
			out.add(pairs, id, stretches, profiles)
		}
	}
	out.free, out.quiet, out.rush = freeFlow(pairs)
	s.store.Logf("traffic: %d lines, %d stop pairs at free flow, in %s", len(out.lines), len(out.free), time.Since(started).Round(time.Millisecond))
	return out, nil
}

func (m *measured) add(pairs map[pair]*secStats, id string, stretches []live.Stretch, profiles []live.SegmentProfile) {
	for _, p := range profiles {
		if p.MedianSec <= 0 || p.SampleCount <= 0 || p.Hour < 0 || p.Hour >= HoursPerDay {
			continue
		}
		k := pair{p.FromStopID, p.ToStopID}
		if pairs[k] == nil {
			pairs[k] = newSecStats()
		}
		pairs[k].add(cell{p.DayType, p.Hour}, p.MedianSec, p.SampleCount)
	}
	for _, st := range stretches {
		if len(st.Points) < 2 {
			continue
		}
		m.lines[id] = append(m.lines[id], &measuredStretch{
			pair: pair{st.FromStopID, st.ToStopID},
			path: geo.EncodePolyline(st.Points, geo.PolylinePrecision),
		})
	}
}

func percent(ratio float64) int { return int(math.Round(ratio * 100)) }

func band(pace int, bands []int) int {
	n := 0
	for _, b := range bands {
		if pace >= b {
			n++
		}
	}
	return n
}
