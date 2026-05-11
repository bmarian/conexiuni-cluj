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

func StartWarmup(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	loc := tranzyClient.Location()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("warmup: recovered from panic: %v", r)
			}
		}()

		for {
			runWarmup(tranzyClient, ctpCjClient, cacheTimes)
			next := nextWarmupAt(time.Now().In(loc))
			log.Printf("warmup: next pass at %s (in %s)", next.Format(time.RFC3339), time.Until(next).Round(time.Minute))
			time.Sleep(time.Until(next))
		}
	}()
}

func nextWarmupAt(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func runWarmup(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) {
	start := time.Now()
	log.Println("warmup: starting")

	Availability.ResetForNewPass()

	phase := time.Now()
	routes, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{})
	if err != nil {
		log.Printf("warmup: routes failed, aborting: %v", err)
		return
	}
	log.Printf("warmup: routes loaded (%d) in %s", len(routes), time.Since(phase).Round(time.Millisecond))

	phase = time.Now()
	stops, err := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{})
	if err != nil {
		log.Printf("warmup: stops failed, aborting: %v", err)
		return
	}
	log.Printf("warmup: stops loaded (%d) in %s", len(stops), time.Since(phase).Round(time.Millisecond))

	phase = time.Now()
	apiStopTimes, err := getAPIStopTimes(tranzyClient, cacheTimes.TranzyCacheShelfLife, APIStopTimeFilter{})
	if err != nil {
		log.Printf("warmup: api_stop_times failed, aborting: %v", err)
		return
	}
	log.Printf("warmup: api_stop_times loaded (%d rows) in %s", len(apiStopTimes), time.Since(phase).Round(time.Millisecond))

	log.Printf("warmup: fanning out stop_times + timetables for %d routes (CTP limiter ~1/s)", len(routes))

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
			tt, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, rsn)
			if err != nil {
				log.Printf("warmup: timetable %s: %v", rsn, err)
				timetablesFail.Add(1)
			} else if tt != nil && (len(tt.Weekdays.Entries) > 0 || len(tt.Saturday.Entries) > 0 || len(tt.Sunday.Entries) > 0) {
				// Only mark routes with at least one departure.
				Availability.MarkRouteHasTimetable(rsn)
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
		info, err := GetStopInfo(tranzyClient, ctpCjClient, cacheTimes, StopFilter{StopID: &stopID})
		if err != nil {
			log.Printf("warmup: stop_info %d: %v", stopID, err)
			failed.Add(1)
		} else {
			warmed.Add(1)
			if info != nil && len(info.ShapesInfo) > 0 {
				Availability.MarkStopHasBuses(stopID)
			}
		}
		processed.Add(1)
	}
	close(stopsDone)
	log.Printf("warmup: stop_info done in %s (%d/%d warmed, %d failed)",
		time.Since(stopStart).Round(time.Millisecond), warmed.Load(), totalStops, failed.Load())

	Availability.MarkReady()
	log.Printf("warmup: availability registry ready (routes-with-timetable + stops-with-buses now filter /api/routes and /api/stops)")

	InvalidateSitemap()

	if err := BuildGTFSZip(tranzyClient, ctpCjClient, cacheTimes); err != nil {
		log.Printf("warmup: GTFS zip build failed: %v", err)
	}

	TriggerOTPRebuild()

	log.Printf("warmup: completed full pass in %s", time.Since(start).Round(time.Millisecond))
}

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
