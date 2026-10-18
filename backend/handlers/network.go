package handlers

import (
	"bytes"
	"compress/gzip"
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
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

// How far past its outermost line a city's map pans and a location still counts as in the city.
const cityAreaMarginMeters = 10_000

func (city *City) BuildNetworkBundle() (*models.NetworkBundle, *Bounds, error) {
	routes, err := city.GetRoutes(RouteFilter{})
	if err != nil {
		return nil, nil, fmt.Errorf("routes: %w", err)
	}
	if city.availability.IsReady() {
		kept := make([]models.Route, 0, len(routes))
		for _, r := range routes {
			if city.availability.RouteHasTimetable(r.RouteShortName) {
				kept = append(kept, r)
			}
		}
		routes = kept
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].RouteID < routes[j].RouteID })

	allStops, err := city.GetStops(StopFilter{})
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
			out[i] = city.buildNetworkRoute(route)
		}(i, route)
	}
	wg.Wait()

	shapes := city.tripShapes(out)
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
	for _, s := range allStops {
		if used[s.StopID] || !city.availability.IsReady() || city.availability.StopHasBuses(s.StopID) {
			stops = append(stops, s)
		}
	}
	sort.Slice(stops, func(i, j int) bool { return stops[i].StopID < stops[j].StopID })

	bundle := &models.NetworkBundle{Routes: out, Stops: stops, Rules: models.NetworkRules{LateGraceMin: city.learnLiveRules().graceSec / 60}}
	bundle.ScheduleDays, bundle.DayNotes = city.networkScheduleDays(time.Now())
	bundle.Version, bundle.StaticVersion = networkVersions(bundle)
	bundle.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return bundle, areaAround(shapes), nil
}

func (city *City) buildNetworkRoute(route models.Route) models.NetworkRoute {
	out := models.NetworkRoute{
		RouteID: route.RouteID, AgencyID: route.AgencyID, RouteShortName: route.RouteShortName,
		RouteLongName: route.RouteLongName, RouteType: route.RouteType, RouteDesc: route.RouteDesc,
		RouteColor: route.RouteColor, Trips: []models.NetworkTrip{}, Offsets: map[string][][]int{},
	}
	rsn := route.RouteShortName
	if timetable, err := city.GetTimetable(rsn); err == nil && timetable != nil {
		out.Timetable = timetable
	}
	base, routeID, err := city.getBaseStopTimes(StopTimeFilter{RouteShortName: &rsn})
	if err != nil {
		city.log("network: stop times for %s: %v", rsn, err)
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

	loc := city.Location
	today := time.Now().In(loc)
	for _, dayType := range networkDayTypes {
		day := city.dayOfType(today, dayType)
		hours := make([][]int, 24)
		for hour := range hours {
			rows := base
			if routeID != 0 {
				rows = city.applySegmentProfilesToStopTimes(base, routeID, time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, loc))
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

func (city *City) tripShapes(routes []models.NetworkRoute) []models.Shape {
	var ids []string
	for _, r := range routes {
		for _, t := range r.Trips {
			ids = append(ids, t.TripID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	points, err := city.GetShapes(ids)
	if err != nil {
		city.log("network: shapes: %v", err)
	}
	return points
}

func addShapeHashes(routes []models.NetworkRoute, points []models.Shape) {
	polylines := shapePolylines(points)
	for i := range routes {
		for j := range routes[i].Trips {
			if p, ok := polylines[routes[i].Trips[j].TripID]; ok {
				routes[i].Trips[j].ShapeHash = shapeHash(p)
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

func (city *City) buildNetwork() (*builtNetwork, error) {
	city.networkBuildMu.Lock()
	defer city.networkBuildMu.Unlock()
	period := ProfileSnapshotPeriod()
	city.networkCache.Lock()
	current := city.networkCache.current
	city.networkCache.Unlock()
	if current != nil && current.period == period {
		return current, nil
	}

	started := time.Now()
	bundle, bounds, err := city.BuildNetworkBundle()
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
	city.networkCache.Lock()
	city.networkCache.current = built
	city.networkCache.Unlock()
	city.log("network: built version=%s static=%s routes=%d json=%dKB gzip=%dKB in %s",
		built.version.Version, built.version.StaticVersion, len(bundle.Routes), len(data)/1024, len(built.gzipped)/1024,
		time.Since(started).Round(time.Millisecond))
	return built, nil
}

func (city *City) currentNetwork() (*builtNetwork, error) {
	city.networkCache.Lock()
	current := city.networkCache.current
	stale := current != nil && current.period != ProfileSnapshotPeriod()
	if stale && !city.networkCache.building {
		city.networkCache.building = true
		go func() {
			defer func() {
				city.networkCache.Lock()
				city.networkCache.building = false
				city.networkCache.Unlock()
			}()
			if _, err := city.buildNetwork(); err != nil {
				city.log("network: rebuild failed: %v", err)
			}
		}()
	}
	city.networkCache.Unlock()
	if current != nil {
		return current, nil
	}
	return city.buildNetwork()
}

func (city *City) WarmNetwork() {
	if _, err := city.buildNetwork(); err != nil {
		city.log("network: warm build failed: %v", err)
	}
}

func (city *City) NetworkHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		built, err := city.currentNetwork()
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
}

func (city *City) NetworkVersionHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		built, err := city.currentNetwork()
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(built.version)
	}
}
