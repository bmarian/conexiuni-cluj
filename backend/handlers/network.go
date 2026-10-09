package handlers

import (
	"bytes"
	"compress/gzip"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

var networkDayTypes = []string{"weekday", "saturday", "sunday"}

func BuildNetworkBundle(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) (*models.NetworkBundle, error) {
	routes, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{})
	if err != nil {
		return nil, fmt.Errorf("routes: %w", err)
	}
	if Availability.IsReady() {
		kept := make([]models.Route, 0, len(routes))
		for _, r := range routes {
			if Availability.RouteHasTimetable(r.RouteShortName) {
				kept = append(kept, r)
			}
		}
		routes = kept
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].RouteID < routes[j].RouteID })

	allStops, err := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{})
	if err != nil {
		return nil, fmt.Errorf("stops: %w", err)
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
			out[i] = buildNetworkRoute(tranzyClient, ctpCjClient, cacheTimes, route)
		}(i, route)
	}
	wg.Wait()

	addShapeHashes(tranzyClient, cacheTimes, out)

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
		if used[s.StopID] || !Availability.IsReady() || Availability.StopHasBuses(s.StopID) {
			stops = append(stops, s)
		}
	}
	sort.Slice(stops, func(i, j int) bool { return stops[i].StopID < stops[j].StopID })

	bundle := &models.NetworkBundle{Routes: out, Stops: stops}
	bundle.Version, bundle.StaticVersion = networkVersions(bundle)
	bundle.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return bundle, nil
}

func buildNetworkRoute(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes, route models.Route) models.NetworkRoute {
	out := models.NetworkRoute{
		RouteID: route.RouteID, AgencyID: route.AgencyID, RouteShortName: route.RouteShortName,
		RouteLongName: route.RouteLongName, RouteType: route.RouteType, RouteDesc: route.RouteDesc,
		RouteColor: route.RouteColor, Trips: []models.NetworkTrip{}, Offsets: map[string][][]int{},
	}
	rsn := route.RouteShortName
	if timetable, err := GetTimetable(ctpCjClient, tranzyClient, cacheTimes, rsn); err == nil && timetable != nil {
		out.Timetable = timetable
	}
	base, routeID, err := getBaseStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn})
	if err != nil {
		log.Printf("network: stop times for %s: %v", rsn, err)
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

	loc := tranzyClient.Location()
	today := time.Now().In(loc)
	for _, dayType := range networkDayTypes {
		day := dayOfType(today, dayType)
		hours := make([][]int, 24)
		for hour := range hours {
			rows := base
			if routeID != 0 {
				rows = applySegmentProfilesToStopTimes(base, routeID, time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, loc))
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

func addShapeHashes(tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, routes []models.NetworkRoute) {
	var ids []string
	for _, r := range routes {
		for _, t := range r.Trips {
			ids = append(ids, t.TripID)
		}
	}
	if len(ids) == 0 {
		return
	}
	points, err := GetShapes(tranzyClient, cacheTimes.TranzyCacheShelfLife, ShapeFilter{ShapeIDs: ids})
	if err != nil {
		log.Printf("network: shapes: %v", err)
		return
	}
	polylines := shapePolylines(points)
	for i := range routes {
		for j := range routes[i].Trips {
			if p, ok := polylines[routes[i].Trips[j].TripID]; ok {
				routes[i].Trips[j].ShapeHash = shapeHash(p)
			}
		}
	}
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
	}{bundle.Routes, bundle.Stops})
	withoutOffsets := make([]models.NetworkRoute, len(bundle.Routes))
	for i, r := range bundle.Routes {
		r.Offsets = nil
		withoutOffsets[i] = r
	}
	static = hash(struct {
		R []models.NetworkRoute
		S []models.Stop
	}{withoutOffsets, bundle.Stops})
	return version, static
}

type builtNetwork struct {
	period  int64
	json    []byte
	gzipped []byte
	version models.NetworkVersion
}

var networkCache struct {
	sync.Mutex
	current  *builtNetwork
	building bool
}

var networkBuildMu sync.Mutex

func buildNetwork(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) (*builtNetwork, error) {
	networkBuildMu.Lock()
	defer networkBuildMu.Unlock()
	period := ProfileSnapshotPeriod()
	networkCache.Lock()
	current := networkCache.current
	networkCache.Unlock()
	if current != nil && current.period == period {
		return current, nil
	}

	started := time.Now()
	bundle, err := BuildNetworkBundle(tranzyClient, ctpCjClient, cacheTimes)
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
	}
	if current != nil && current.version.Version == built.version.Version {
		built = &builtNetwork{period: period, json: current.json, gzipped: current.gzipped, version: current.version}
	}
	networkCache.Lock()
	networkCache.current = built
	networkCache.Unlock()
	log.Printf("network: built version=%s static=%s routes=%d json=%dKB gzip=%dKB in %s",
		built.version.Version, built.version.StaticVersion, len(bundle.Routes), len(data)/1024, len(built.gzipped)/1024,
		time.Since(started).Round(time.Millisecond))
	return built, nil
}

func currentNetwork(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) (*builtNetwork, error) {
	networkCache.Lock()
	current := networkCache.current
	stale := current != nil && current.period != ProfileSnapshotPeriod()
	if stale && !networkCache.building {
		networkCache.building = true
		go func() {
			defer func() {
				networkCache.Lock()
				networkCache.building = false
				networkCache.Unlock()
			}()
			if _, err := buildNetwork(tranzyClient, ctpCjClient, cacheTimes); err != nil {
				log.Printf("network: rebuild failed: %v", err)
			}
		}()
	}
	networkCache.Unlock()
	if current != nil {
		return current, nil
	}
	return buildNetwork(tranzyClient, ctpCjClient, cacheTimes)
}

func WarmNetwork(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	if _, err := buildNetwork(tranzyClient, ctpCjClient, cacheTimes); err != nil {
		log.Printf("network: warm build failed: %v", err)
	}
}

func NetworkHandler(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) fiber.Handler {
	return func(c fiber.Ctx) error {
		built, err := currentNetwork(tranzyClient, ctpCjClient, cacheTimes)
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

func NetworkVersionHandler(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) fiber.Handler {
	return func(c fiber.Ctx) error {
		built, err := currentNetwork(tranzyClient, ctpCjClient, cacheTimes)
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(built.version)
	}
}
