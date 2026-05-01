package handlers

import (
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/planner"
	"conexiuni-cluj/services/tranzy"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

var planBuildOnce sync.Mutex

func handlePlanRoutes(c fiber.Ctx) error {
	g := planner.Active()
	if g == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "planner not ready"})
	}

	fromLatS := c.Query("from_lat")
	fromLngS := c.Query("from_lng")
	toLatS := c.Query("to_lat")
	toLngS := c.Query("to_lng")
	if fromLatS == "" || fromLngS == "" || toLatS == "" || toLngS == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from_lat, from_lng, to_lat, to_lng are required"})
	}

	fromLat, err := strconv.ParseFloat(fromLatS, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from_lat"})
	}
	fromLng, err := strconv.ParseFloat(fromLngS, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from_lng"})
	}
	toLat, err := strconv.ParseFloat(toLatS, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to_lat"})
	}
	toLng, err := strconv.ParseFloat(toLngS, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to_lng"})
	}

	candidates := g.Plan(planner.PlanRequest{
		OriginLat: fromLat,
		OriginLon: fromLng,
		DestLat:   toLat,
		DestLon:   toLng,
	})
	resp := g.BuildResponse(candidates)
	c.Set("Cache-Control", "no-store")
	return c.JSON(resp)
}

// RebuildPlannerGraph collects the data needed by the planner from the existing
// caches and atomically swaps in a fresh Graph. Safe to call after warmup or on
// cache invalidation. Runs synchronously; callers can wrap in a goroutine.
func RebuildPlannerGraph(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	if !planBuildOnce.TryLock() {
		return
	}
	defer planBuildOnce.Unlock()

	start := time.Now()
	stops, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{})
	if err != nil {
		log.Printf("planner: stops fetch failed: %v", err)
		return
	}
	routes, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{})
	if err != nil {
		log.Printf("planner: routes fetch failed: %v", err)
		return
	}

	stopTimesByRoute := make(map[string][]models.StopTime, len(routes))
	timetablesByRoute := make(map[string]models.Timetable, len(routes))
	for _, r := range routes {
		rsn := r.RouteShortName
		sts, err := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn})
		if err != nil {
			continue
		}
		stopTimesByRoute[rsn] = sts
		tt, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, rsn)
		if err != nil || tt == nil {
			continue
		}
		timetablesByRoute[rsn] = *tt
	}

	g := planner.Build(stops, routes, stopTimesByRoute, timetablesByRoute)
	planner.SetActive(g)
	log.Printf("planner: graph rebuilt in %s (stops=%d routes=%d)", time.Since(start).Round(time.Millisecond), len(stops), len(stopTimesByRoute))
}
