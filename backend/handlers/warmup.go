package handlers

import (
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"log"
	"sync"
	"time"
)

// StartWarmup runs a background goroutine that pre-fetches the caches that
// make cold `/api/stop_info` requests slow. The CTP timetable rate limiter
// (1 req/s) means a heavy stop with ~12 routes pays ~10s of serialized waits
// on first request; warming up-front eats that cost during startup instead.
//
// Everything delegates to the normal Get* handlers, which are no-ops when
// their cache is still valid. The warmup never fails the server — errors
// are logged and skipped.
func StartWarmup(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("warmup: recovered from panic: %v", r)
			}
		}()
		runWarmup(tranzyClient, ctpCjClient, cacheTimes)
	}()
}

func runWarmup(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	start := time.Now()
	log.Println("warmup: starting")

	routes, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{})
	if err != nil {
		log.Printf("warmup: routes failed, aborting: %v", err)
		return
	}

	stops, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{})
	if err != nil {
		log.Printf("warmup: stops failed, aborting: %v", err)
		return
	}

	if _, err := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{}); err != nil {
		log.Printf("warmup: api_stop_times failed, aborting: %v", err)
		return
	}

	// Per-route warmup: stop_times is Tranzy-backed (rate-limited by the Tranzy
	// client), timetable is CTP-backed (rate-limited at 1/s). Kick them off in
	// parallel goroutines — both clients serialize internally via their
	// limiters, so there's no risk of overrun, and the two streams run
	// concurrently.
	var wg sync.WaitGroup
	for _, r := range routes {
		rsn := r.RouteShortName
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn}); err != nil {
				log.Printf("warmup: stop_times %s: %v", rsn, err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, rsn); err != nil {
				log.Printf("warmup: timetable %s: %v", rsn, err)
			}
		}()
	}
	wg.Wait()
	log.Printf("warmup: routes/timetables done in %s (%d routes)", time.Since(start), len(routes))

	// Per-stop stop_info warmup. At this point every route-scoped dependency
	// is in DB cache, so each GetStopInfo follows the warm path (DB only). Run
	// sequentially — the DB reads are fast and we avoid hammering the cache
	// mutex / sqlite writer.
	stopStart := time.Now()
	warmed := 0
	for _, s := range stops {
		stopID := s.StopID
		if _, err := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &stopID}); err != nil {
			log.Printf("warmup: stop_info %d: %v", stopID, err)
			continue
		}
		warmed++
	}
	log.Printf("warmup: stop_info done in %s (%d/%d stops, total %s)",
		time.Since(stopStart), warmed, len(stops), time.Since(start))
}
