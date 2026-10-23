// Package network builds /network: every line with its timetable, stops and learned times for each hour of each kind of day, which clients build their pages from.
package network

import (
	"bytes"
	"compress/gzip"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/geo"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"conexiuni-cluj/transit"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

var networkDayTypes = calendar.ScheduleDays

const cityAreaMarginMeters = 10_000

func (s *Service) bundle() (*models.NetworkBundle, *Bounds, error) {
	routes, err := s.data.Routes(transit.RouteFilter{})
	if err != nil {
		return nil, nil, fmt.Errorf("routes: %w", err)
	}
	if s.offered.IsReady() {
		kept := make([]models.Route, 0, len(routes))
		for _, r := range routes {
			if s.offered.RouteHasTimetable(r.RouteShortName) {
				kept = append(kept, r)
			}
		}
		routes = kept
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].RouteID < routes[j].RouteID })

	allStops, err := s.data.Stops(transit.StopFilter{})
	if err != nil {
		return nil, nil, fmt.Errorf("stops: %w", err)
	}

	out := make([]models.NetworkRoute, len(routes))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, route := range routes {
		wg.Add(1)
		go func(i int, route models.Route) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = s.buildNetworkRoute(route)
		}(i, route)
	}
	wg.Wait()

	shapes := s.tripShapes(out)
	addShapeHashes(out, shapes)

	used := make(map[int]bool)
	for _, r := range out {
		for _, t := range r.Trips {
			for _, id := range t.StopIDs {
				used[id] = true
			}
		}
	}
	stops := make([]models.Stop, 0, len(allStops))
	for _, stop := range allStops {
		if used[stop.StopID] || !s.offered.IsReady() || s.offered.StopHasBuses(stop.StopID) {
			stops = append(stops, stop)
		}
	}
	sort.Slice(stops, func(i, j int) bool { return stops[i].StopID < stops[j].StopID })

	bundle := &models.NetworkBundle{Routes: out, Stops: stops, Rules: models.NetworkRules{LateGraceMin: s.timing.GraceSec() / 60}}
	bundle.ScheduleDays, bundle.DayNotes = s.timing.ScheduleDays(time.Now())
	bundle.Version, bundle.StaticVersion = networkVersions(bundle)
	bundle.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return bundle, areaAround(shapes), nil
}

func (s *Service) buildNetworkRoute(route models.Route) models.NetworkRoute {
	out := models.NetworkRoute{
		RouteID: route.RouteID, AgencyID: route.AgencyID, RouteShortName: route.RouteShortName,
		RouteLongName: route.RouteLongName, RouteType: route.RouteType, RouteDesc: route.RouteDesc,
		RouteColor: route.RouteColor, Trips: []models.NetworkTrip{}, Offsets: map[string][][]int{},
	}
	rsn := route.RouteShortName
	if timetable, err := s.data.Timetable(rsn); err == nil && timetable != nil {
		out.Timetable = timetable
	}
	base, routeID, err := s.data.BaseStopTimes(transit.StopTimeFilter{RouteShortName: &rsn})
	if err != nil {
		s.store.Logf("network: stop times for %s: %v", rsn, err)
		return out
	}
	for _, st := range base {
		last := len(out.Trips) - 1
		if last < 0 || out.Trips[last].TripID != st.TripID {
			out.Trips = append(out.Trips, models.NetworkTrip{TripID: st.TripID, StopIDs: []int{}, StopSequences: []int{}})
			last++
		}
		out.Trips[last].StopIDs = append(out.Trips[last].StopIDs, st.StopID)
		out.Trips[last].StopSequences = append(out.Trips[last].StopSequences, st.StopSequence)
	}

	loc := s.loc
	today := time.Now().In(loc)
	for _, dayType := range networkDayTypes {
		day := s.timing.DayOfType(today, dayType)
		hours := make([][]int, 24)
		for hour := range hours {
			rows := base
			if routeID != 0 {
				rows = s.timing.TimedStopTimes(base, routeID, time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, loc))
			}
			offsets := make([]int, len(rows))
			for i, st := range rows {
				offsets[i] = int(math.Round(st.OffsetArrivalTime))
			}
			hours[hour] = offsets
		}
		out.Offsets[dayType] = hours
	}
	return out
}

func (s *Service) tripShapes(routes []models.NetworkRoute) []models.Shape {
	var ids []string
	for _, r := range routes {
		for _, t := range r.Trips {
			ids = append(ids, t.TripID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	points, err := s.data.Shapes(ids)
	if err != nil {
		s.store.Logf("network: shapes: %v", err)
	}
	return points
}

func addShapeHashes(routes []models.NetworkRoute, points []models.Shape) {
	polylines := geo.Polylines(points)
	for i := range routes {
		for j := range routes[i].Trips {
			if p, ok := polylines[routes[i].Trips[j].TripID]; ok {
				routes[i].Trips[j].ShapeHash = geo.ShapeHash(p)
			}
		}
	}
}

func areaAround(points []models.Shape) *Bounds {
	if len(points) == 0 {
		return nil
	}
	box := router.BBox{MinLat: math.Inf(1), MinLon: math.Inf(1), MaxLat: math.Inf(-1), MaxLon: math.Inf(-1)}
	for _, p := range points {
		box.MinLat, box.MaxLat = math.Min(box.MinLat, p.ShapePtLat), math.Max(box.MaxLat, p.ShapePtLat)
		box.MinLon, box.MaxLon = math.Min(box.MinLon, p.ShapePtLon), math.Max(box.MaxLon, p.ShapePtLon)
	}
	box = box.Grow(cityAreaMarginMeters)
	return &Bounds{South: box.MinLat, West: box.MinLon, North: box.MaxLat, East: box.MaxLon}
}

func networkVersions(bundle *models.NetworkBundle) (version, static string) {
	hash := func(v any) string {
		data, _ := json.Marshal(v)
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:8])
	}
	version = hash(struct {
		R []models.NetworkRoute
		S []models.Stop
		D map[string]string
		N map[string]models.DayNote
	}{bundle.Routes, bundle.Stops, bundle.ScheduleDays, bundle.DayNotes})
	withoutOffsets := make([]models.NetworkRoute, len(bundle.Routes))
	for i, r := range bundle.Routes {
		r.Offsets = nil
		withoutOffsets[i] = r
	}
	static = hash(struct {
		R []models.NetworkRoute
		S []models.Stop
		D map[string]string
		N map[string]models.DayNote
	}{withoutOffsets, bundle.Stops, bundle.ScheduleDays, bundle.DayNotes})
	return version, static
}

type builtNetwork struct {
	period  int64
	json    []byte
	gzipped []byte
	version models.NetworkVersion
	bounds  *Bounds
}

func (s *Service) buildNetwork() (*builtNetwork, error) {
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	period := s.timing.Period()
	s.mu.Lock()
	current := s.current
	s.mu.Unlock()
	if current != nil && current.period == period {
		return current, nil
	}

	started := time.Now()
	bundle, bounds, err := s.bundle()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	var gz bytes.Buffer
	w, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	_, _ = w.Write(data)
	_ = w.Close()
	built := &builtNetwork{
		period:  period,
		json:    data,
		gzipped: gz.Bytes(),
		version: models.NetworkVersion{Version: bundle.Version, StaticVersion: bundle.StaticVersion, GeneratedAt: bundle.GeneratedAt},
		bounds:  bounds,
	}
	if current != nil && current.version.Version == built.version.Version {
		built = &builtNetwork{period: period, json: current.json, gzipped: current.gzipped, version: current.version, bounds: bounds}
	}
	s.mu.Lock()
	s.current = built
	s.mu.Unlock()
	s.store.Logf("network: built version=%s static=%s routes=%d json=%dKB gzip=%dKB in %s",
		built.version.Version, built.version.StaticVersion, len(bundle.Routes), len(data)/1024, len(built.gzipped)/1024,
		time.Since(started).Round(time.Millisecond))
	return built, nil
}

func (s *Service) currentNetwork() (*builtNetwork, error) {
	s.mu.Lock()
	current := s.current
	stale := current != nil && current.period != s.timing.Period()
	if stale && !s.building {
		s.building = s.Runner().Go(func() {
			defer func() {
				s.mu.Lock()
				s.building = false
				s.mu.Unlock()
			}()
			if _, err := s.buildNetwork(); err != nil {
				s.store.Logf("network: rebuild failed: %v", err)
			}
		})
	}
	s.mu.Unlock()
	if current != nil {
		return current, nil
	}
	return s.buildNetwork()
}

// Warm builds the bundle now, so the first client does not wait for it.
func (s *Service) Warm() {
	if _, err := s.buildNetwork(); err != nil {
		s.store.Logf("network: warm build failed: %v", err)
	}
}

// Current is the bundle clients get now, read back from its JSON, and its version.
func (s *Service) Current() (*models.NetworkBundle, string, error) {
	built, err := s.currentNetwork()
	if err != nil {
		return nil, "", err
	}
	var bundle models.NetworkBundle
	if err := json.Unmarshal(built.json, &bundle); err != nil {
		return nil, "", err
	}
	return &bundle, built.version.Version, nil
}

// Version is the version Current would return, without reading the bundle.
func (s *Service) Version() (string, error) {
	built, err := s.currentNetwork()
	if err != nil {
		return "", err
	}
	return built.version.Version, nil
}

func (s *Service) serve(c fiber.Ctx) error {
	built, err := s.currentNetwork()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	etag := `"` + built.version.Version + `"`
	c.Set("ETag", etag)
	c.Set("Cache-Control", "no-cache")
	c.Set("Vary", "Accept-Encoding")
	for _, tag := range strings.Split(c.Get("If-None-Match"), ",") {
		if strings.TrimPrefix(strings.TrimSpace(tag), "W/") == etag {
			return c.SendStatus(fiber.StatusNotModified)
		}
	}
	c.Set("Content-Type", "application/json")
	if strings.Contains(c.Get("Accept-Encoding"), "gzip") {
		c.Set("Content-Encoding", "gzip")
		return c.Send(built.gzipped)
	}
	return c.Send(built.json)
}

func (s *Service) serveVersion(c fiber.Ctx) error {
	built, err := s.currentNetwork()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(built.version)
}
