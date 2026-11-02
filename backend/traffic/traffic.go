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
	LiveWindow = 30 * time.Minute
	// How many usual buses a stretch's recent ones weigh against: the prior the live-times replay settled on.
	livePrior = 4.0
	liveEvery = time.Minute
)

type Timing interface {
	Stretches(tripID string) ([]live.Stretch, bool)
	SegmentProfiles(routeID, directionID int) ([]live.SegmentProfile, error)
	RecentStretchTimes(since time.Time) ([]live.RecentStretch, error)
	ProfileDay(t time.Time) string
	Period() int64
}

type Now struct {
	Lines map[string][]Stretch `json:"lines"`
}

type Stretch struct {
	Path string `json:"path"`
	Band int `json:"band"`
}

type Service struct {
	loc     *time.Location
	store   *database.Store
	data    *transit.Data
	offered *transit.Availability
	timing  Timing
	// A timetable-only city has no bus timings to colour by.
	realtime bool
	Runner func() *jobs.Runner

	mu       sync.Mutex
	period   int64
	building bool
	all      *measured
	rated    map[cell]*ratedNow
}

type ratedNow struct {
	Now
	at time.Time
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
	s.rebuild()
	s.mu.Unlock()
}

func (s *Service) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.all != nil
}

func (s *Service) serve(c fiber.Ctx) error {
	now, err := s.now(time.Now().In(s.loc))
	if err != nil {
		c.Set("Cache-Control", "no-store")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	out := Now{Lines: make(map[string][]Stretch)}
	for _, id := range strings.Split(c.Query("trip_ids"), ",") {
		if stretches, ok := now.Lines[id]; ok {
			out.Lines[id] = stretches
		}
	}
	c.Set("Cache-Control", web.Revalidate)
	return c.JSON(out)
}

var (
	errBuilding   = errors.New("the traffic map is still being built")
	errNoTimings  = errors.New("no live buses to measure traffic from")
	errUnmeasured = errors.New("nothing measured at this hour yet")
)

func (s *Service) now(at time.Time) (*ratedNow, error) {
	if !s.realtime {
		return nil, errNoTimings
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.all == nil || s.timing.Period() != s.period {
		s.rebuild()
	}
	if s.all == nil {
		return nil, errBuilding
	}
	k := cell{s.timing.ProfileDay(at), at.Hour()}
	if rated := s.rated[k]; rated != nil && at.Sub(rated.at) < liveEvery {
		return rated, nil
	}
	now, err := s.all.now(k, s.recent(at))
	if err != nil {
		return nil, err
	}
	s.rated[k] = &ratedNow{Now: now, at: at}
	return s.rated[k], nil
}

func (s *Service) recent(at time.Time) map[stretchKey]live.RecentStretch {
	rows, err := s.timing.RecentStretchTimes(at.Add(-LiveWindow))
	if err != nil {
		s.store.Logf("traffic: recent timings: %v", err)
		return nil
	}
	out := make(map[stretchKey]live.RecentStretch, len(rows))
	for _, r := range rows {
		out[stretchKey{r.RouteID, r.DirectionID, r.FromStopID, r.ToStopID}] = r
	}
	return out
}

// rebuild measures in the background, once at a time; the caller holds the lock.
func (s *Service) rebuild() {
	if s.building {
		return
	}
	s.building = true
	period := s.timing.Period()
	started := s.Runner().Go(func() {
		all, err := s.measure()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.building = false
		if err != nil {
			s.store.Logf("traffic: build failed: %v", err)
			return
		}
		s.all, s.period, s.rated = all, period, make(map[cell]*ratedNow)
	})
	if !started {
		s.building = false
	}
}

type cell struct {
	dayType string
	hour    int
}

type stretchKey struct {
	routeID, directionID, fromStopID, toStopID int
}

type measuredStretch struct {
	key    stretchKey
	path   string
	meters float64
	paceStats
	// Over the whole of each kind of day, the ETA model's own fallback.
	allDay map[string]paceSum
}

type paceSum struct {
	pace    float64
	samples int
}

// By the stretch's profile at k, else the hours either side, else the whole of that kind of day.
func (st *measuredStretch) slowdown(k cell) (float64, bool) {
	if st.free <= 0 {
		return 0, false
	}
	if pace, ok := st.usual(k); ok {
		return pace / st.free, true
	}
	sum, n := 0.0, 0
	for _, h := range []int{k.hour - 1, k.hour + 1} {
		near := cell{k.dayType, (h + HoursPerDay) % HoursPerDay}
		if pace, ok := st.usual(near); ok {
			sum += pace * float64(st.samples[near])
			n += st.samples[near]
		}
	}
	if n > 0 {
		return sum / float64(n) / st.free, true
	}
	if day, ok := st.allDay[k.dayType]; ok {
		return day.pace / st.free, true
	}
	return 0, false
}

type measured struct {
	bands []int
	lines map[string][]*measuredStretch
}

func (m *measured) now(k cell, recent map[stretchKey]live.RecentStretch) (Now, error) {
	out := Now{Lines: make(map[string][]Stretch, len(m.lines))}
	for id, line := range m.lines {
		ratio := make(map[stretchKey]float64, len(line))
		for _, st := range line {
			r, ok := st.slowdown(k)
			if rec, has := recent[st.key]; has && rec.Samples > 0 && st.free > 0 {
				if !ok {
					r = st.typical / st.free
				}
				seen := rec.MeanSec / st.meters / st.free
				ratio[st.key] = (livePrior*r + seen*float64(rec.Samples)) / (livePrior + float64(rec.Samples))
			} else if ok {
				ratio[st.key] = r
			}
		}
		fillBetween(line, ratio)
		for _, st := range line {
			if r, ok := ratio[st.key]; ok {
				out.Lines[id] = append(out.Lines[id], Stretch{Path: st.path, Band: band(percent(r), m.bands)})
			}
		}
	}
	if len(out.Lines) == 0 {
		return Now{}, errUnmeasured
	}
	return out, nil
}

func (s *Service) measure() (*measured, error) {
	started := time.Now()
	routes, err := s.data.Routes(transit.RouteFilter{})
	if err != nil {
		return nil, fmt.Errorf("routes: %w", err)
	}
	out := &measured{lines: make(map[string][]*measuredStretch)}
	var stats []*paceStats
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
			byPair := make(map[[2]int][]live.SegmentProfile, len(stretches))
			for _, p := range profiles {
				byPair[[2]int{p.FromStopID, p.ToStopID}] = append(byPair[[2]int{p.FromStopID, p.ToStopID}], p)
			}
			for _, st := range stretches {
				meters := geo.ShapeDistance(st.Points, 0, len(st.Points)-1)
				if meters <= 0 {
					continue
				}
				key := stretchKey{r.RouteID, direction, st.FromStopID, st.ToStopID}
				learned := learn(key, geo.EncodePolyline(st.Points, geo.PolylinePrecision), meters, byPair[[2]int{st.FromStopID, st.ToStopID}])
				if learned.typical > 0 {
					stats = append(stats, &learned.paceStats)
				}
				out.lines[id] = append(out.lines[id], learned)
			}
		}
	}
	out.bands = splits(rateFreeFlow(stats))
	s.store.Logf("traffic: %d lines, %d stretches learned, bands %v, in %s", len(out.lines), len(stats), out.bands, time.Since(started).Round(time.Millisecond))
	return out, nil
}

func learn(key stretchKey, path string, meters float64, profiles []live.SegmentProfile) *measuredStretch {
	st := &measuredStretch{key: key, path: path, meters: meters, paceStats: paceStats{pace: map[cell]float64{}, samples: map[cell]int{}}, allDay: map[string]paceSum{}}
	for _, p := range profiles {
		if p.MedianSec <= 0 || p.SampleCount <= 0 || p.Hour >= HoursPerDay {
			continue
		}
		if p.Hour == live.AllDay {
			st.allDay[p.DayType] = paceSum{p.MedianSec / meters, p.SampleCount}
			continue
		}
		k := cell{p.DayType, p.Hour}
		st.pace[k] += p.MedianSec / meters * float64(p.SampleCount)
		st.samples[k] += p.SampleCount
	}
	st.typical = st.medianPace(func(cell) bool { return true })
	return st
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
