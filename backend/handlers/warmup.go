package handlers

import (
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// StartWarmup runs a background goroutine that pre-fetches the caches that
// make cold `/api/stop_info` requests slow. The CTP timetable rate limiter
// (1 req/s) means a heavy stop with ~12 routes pays ~10s of serialized waits
// on first request; warming up-front eats that cost during startup instead.
//
// The goroutine runs warmup once immediately, then re-runs it on an interval
// sized to the shortest relevant cache TTL so entries get refreshed just
// before they expire. Everything delegates to the normal Get* handlers, which
// are no-ops when their cache is still valid — so the recurring passes only
// pay cost for entries that are actually near expiry.
//
// The warmup never fails the server — errors are logged and skipped.
func StartWarmup(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("warmup: recovered from panic: %v", r)
			}
		}()

		for {
			runStart := time.Now()
			runWarmup(tranzyClient, ctpCjClient, cacheTimes)
			next := nextWarmupAt(runStart, cacheTimes)
			log.Printf("warmup: next pass at %s (in %s)", next.Format(time.RFC3339), time.Until(next).Round(time.Minute))
			time.Sleep(time.Until(next))
		}
	}()
}

// nextWarmupAt schedules the next warmup at 03:00 local time on the morning
// before the shortest cache TTL would expire, relative to when the current
// warmup started. Minimum 1h ahead so misconfigured short TTLs don't spin.
func nextWarmupAt(runStart time.Time, cacheTimes models.CacheTimes) time.Time {
	candidates := []time.Duration{
		cacheTimes.TimetableCacheShelfLife,
		cacheTimes.StopInfoCacheShelfLife,
		cacheTimes.StopTimeCacheShelfLife,
		cacheTimes.APIStopTimeCacheShelfLife,
		cacheTimes.RouteCacheShelfLife,
		cacheTimes.StopCacheShelfLife,
	}
	shortest := candidates[0]
	for _, c := range candidates[1:] {
		if c > 0 && c < shortest {
			shortest = c
		}
	}

	expiry := runStart.Add(shortest)
	dayBefore := expiry.AddDate(0, 0, -1)
	next := time.Date(dayBefore.Year(), dayBefore.Month(), dayBefore.Day(), 3, 0, 0, 0, dayBefore.Location())

	if min := time.Now().Add(time.Hour); next.Before(min) {
		next = min
	}
	return next
}

func runWarmup(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	start := time.Now()
	log.Println("warmup: starting")

	phase := time.Now()
	routes, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{})
	if err != nil {
		log.Printf("warmup: routes failed, aborting: %v", err)
		return
	}
	log.Printf("warmup: routes loaded (%d) in %s", len(routes), time.Since(phase).Round(time.Millisecond))

	phase = time.Now()
	stops, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{})
	if err != nil {
		log.Printf("warmup: stops failed, aborting: %v", err)
		return
	}
	log.Printf("warmup: stops loaded (%d) in %s", len(stops), time.Since(phase).Round(time.Millisecond))

	phase = time.Now()
	apiStopTimes, err := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{})
	if err != nil {
		log.Printf("warmup: api_stop_times failed, aborting: %v", err)
		return
	}
	log.Printf("warmup: api_stop_times loaded (%d rows) in %s", len(apiStopTimes), time.Since(phase).Round(time.Millisecond))

	log.Printf("warmup: fanning out stop_times + timetables for %d routes (CTP limiter ~1/s)", len(routes))

	// Per-route warmup: stop_times is Tranzy-backed (rate-limited by the Tranzy
	// client), timetable is CTP-backed (rate-limited at 1/s). Kick them off in
	// parallel goroutines — both clients serialize internally via their
	// limiters, so there's no risk of overrun, and the two streams run
	// concurrently.
	phase = time.Now()
	var (
		wg             sync.WaitGroup
		stopTimesOK    atomic.Int32
		stopTimesFail  atomic.Int32
		timetablesOK   atomic.Int32
		timetablesFail atomic.Int32
	)
	total := int32(len(routes))
	progressDone := make(chan struct{})
	go logProgress(progressDone, 5*time.Second, func() string {
		return progressLine("routes", stopTimesOK.Load(), timetablesOK.Load(), total)
	})

	for _, r := range routes {
		rsn := r.RouteShortName
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn}); err != nil {
				log.Printf("warmup: stop_times %s: %v", rsn, err)
				stopTimesFail.Add(1)
			}
			stopTimesOK.Add(1)
		}()
		go func() {
			defer wg.Done()
			if _, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, rsn); err != nil {
				log.Printf("warmup: timetable %s: %v", rsn, err)
				timetablesFail.Add(1)
			}
			timetablesOK.Add(1)
		}()
	}
	wg.Wait()
	close(progressDone)
	log.Printf("warmup: routes/timetables done in %s (stop_times: %d ok / %d failed, timetables: %d ok / %d failed)",
		time.Since(phase).Round(time.Millisecond),
		stopTimesOK.Load()-stopTimesFail.Load(), stopTimesFail.Load(),
		timetablesOK.Load()-timetablesFail.Load(), timetablesFail.Load())

	// Per-stop stop_info warmup. At this point every route-scoped dependency
	// is in DB cache, so each GetStopInfo follows the warm path (DB only). Run
	// sequentially — the DB reads are fast and we avoid hammering the cache
	// mutex / sqlite writer.
	log.Printf("warmup: priming stop_info for %d stops", len(stops))
	stopStart := time.Now()
	totalStops := int32(len(stops))
	var processed, warmed, failed atomic.Int32

	stopsDone := make(chan struct{})
	go logProgress(stopsDone, 5*time.Second, func() string {
		p, w, f := processed.Load(), warmed.Load(), failed.Load()
		return formatProgress("stops", p, totalStops) + formatWarmed(w) + " failed=" + itoa(f)
	})

	for _, s := range stops {
		stopID := s.StopID
		if _, err := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &stopID}); err != nil {
			log.Printf("warmup: stop_info %d: %v", stopID, err)
			failed.Add(1)
		} else {
			warmed.Add(1)
		}
		processed.Add(1)
	}
	close(stopsDone)
	log.Printf("warmup: stop_info done in %s (%d/%d warmed, %d failed)",
		time.Since(stopStart).Round(time.Millisecond), warmed.Load(), totalStops, failed.Load())
	log.Printf("warmup: completed full pass in %s", time.Since(start).Round(time.Millisecond))
}

// logProgress prints `line()` every `interval` until `done` is closed. Runs in
// its own goroutine and exits promptly on close.
func logProgress(done <-chan struct{}, interval time.Duration, line func() string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			log.Printf("warmup: %s", line())
		}
	}
}

func formatProgress(label string, done, total int32) string {
	pct := 0
	if total > 0 {
		pct = int(float64(done) / float64(total) * 100)
	}
	return label + ": " + itoa(done) + "/" + itoa(total) + " (" + itoa(int32(pct)) + "%)"
}

func formatWarmed(n int32) string {
	return " warmed=" + itoa(n)
}

func progressLine(label string, stopTimes, timetables, total int32) string {
	return label + ": stop_times=" + itoa(stopTimes) + "/" + itoa(total) +
		" timetables=" + itoa(timetables) + "/" + itoa(total)
}

func itoa(n int32) string {
	return strconv.Itoa(int(n))
}
