package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	archiveRetention  = 400 * 24 * time.Hour
	archiveSettle     = time.Hour
	archiveStartDelay = 10 * time.Minute
	archiveHour       = 3
	archiveMinute     = 30
	archiveOverdue    = 26 * time.Hour
)

type archiveRun struct {
	At       time.Time
	Took     time.Duration
	Copied   int
	Versions int
	Pruned   int
	Err      string
	Summary  database.ArchiveSummary
}

var lastArchive = struct {
	sync.Mutex
	run *archiveRun
}{}

func StartArchiver(loc *time.Location) {
	if database.Archive == nil {
		return
	}
	go func() {
		time.Sleep(archiveStartDelay)
		for {
			checkObservedSchedule(time.Now())
			runArchive(time.Now())
			tuneVacationWindow(time.Now())
			tuneSeedFill(time.Now())
			restoreSeededWindows(time.Now())
			time.Sleep(time.Until(nextArchiveRun(time.Now(), loc)))
		}
	}()
}

func nextArchiveRun(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), archiveHour, archiveMinute, 0, 0, loc)
	if !next.After(local) {
		next = time.Date(local.Year(), local.Month(), local.Day()+1, archiveHour, archiveMinute, 0, 0, loc)
	}
	return next
}

func runArchive(now time.Time) {
	started := time.Now()
	run := archiveRun{At: now}
	var errs []string
	copied, err := database.CopyToArchive(now.Add(-archiveSettle))
	for _, n := range copied {
		run.Copied += n
	}
	if err != nil {
		errs = append(errs, "copy: "+err.Error())
	}
	if run.Versions, err = snapshotArchiveVersions(now); err != nil {
		errs = append(errs, "versions: "+err.Error())
	}
	if run.Pruned, err = database.PruneArchive(now.Add(-archiveRetention)); err != nil {
		errs = append(errs, "prune: "+err.Error())
	}
	if run.Summary, err = database.SummarizeArchive(); err != nil {
		errs = append(errs, "summary: "+err.Error())
	}
	run.Err = strings.Join(errs, "; ")
	run.Took = time.Since(started)
	if run.Err != "" {
		log.Printf("archive failed: %s", run.Err)
	} else {
		log.Printf("archive: copied %d rows, %d new timetable or stop versions, pruned %d in %s", run.Copied, run.Versions, run.Pruned, run.Took.Round(time.Millisecond))
	}
	lastArchive.Lock()
	lastArchive.run = &run
	lastArchive.Unlock()
}

func lastArchiveRun() *archiveRun {
	lastArchive.Lock()
	defer lastArchive.Unlock()
	return lastArchive.run
}

func snapshotArchiveVersions(now time.Time) (int, error) {
	timetables, err := queryRows(`
		SELECT route_short_name, route_long_name, in_stop_name, out_stop_name, weekdays, saturday, sunday
		FROM timetable`, nil,
		func(rows *sql.Rows) (database.TimetableVersion, error) {
			var v database.TimetableVersion
			err := rows.Scan(&v.RouteShortName, &v.RouteLongName, &v.InStopName, &v.OutStopName, &v.Weekdays, &v.Saturday, &v.Sunday)
			return v, err
		})
	if err != nil {
		return 0, err
	}
	stored, err := database.ArchiveTimetables(timetables, now)
	if err != nil {
		return stored, err
	}
	stopTimes, err := getStopTimesFromDB(StopTimeFilter{})
	if err != nil {
		return stored, err
	}
	trips, err := database.ArchiveTripStops(tripStopsJSON(stopTimes), now)
	return stored + trips, err
}

func archiveTimetable(t *models.Timetable, weekdays, saturday, sunday string) {
	_, err := database.ArchiveTimetables([]database.TimetableVersion{{
		RouteShortName: t.RouteShortName,
		RouteLongName:  t.RouteLongName,
		InStopName:     t.InStopName,
		OutStopName:    t.OutStopName,
		Weekdays:       weekdays,
		Saturday:       saturday,
		Sunday:         sunday,
	}}, time.Now())
	if err != nil {
		log.Printf("archive: timetable %s failed: %v", t.RouteShortName, err)
	}
}

func archiveStopTimes(stopTimes []models.StopTime) {
	if _, err := database.ArchiveTripStops(tripStopsJSON(stopTimes), time.Now()); err != nil {
		log.Printf("archive: stop times failed: %v", err)
	}
}

type archivedStop struct {
	StopID         int     `json:"stop_id"`
	StopSequence   int     `json:"stop_sequence"`
	Offset         float64 `json:"offset_arrival_time"`
	StopHeadsign   string  `json:"stop_headsign"`
	RouteShortName string  `json:"route_short_name"`
	StopLat        float64 `json:"stop_lat"`
	StopLon        float64 `json:"stop_lon"`
}

func tripStopsJSON(stopTimes []models.StopTime) map[string]string {
	bySeq := make(map[string]map[int]archivedStop)
	for _, st := range stopTimes {
		if bySeq[st.TripID] == nil {
			bySeq[st.TripID] = make(map[int]archivedStop)
		}
		bySeq[st.TripID][st.StopSequence] = archivedStop{
			StopID: st.StopID, StopSequence: st.StopSequence, Offset: st.OffsetArrivalTime, StopHeadsign: st.StopHeadsign,
			RouteShortName: st.RouteShortName, StopLat: st.StopLat, StopLon: st.StopLon,
		}
	}
	out := make(map[string]string, len(bySeq))
	for trip, seqs := range bySeq {
		stops := make([]archivedStop, 0, len(seqs))
		for _, s := range seqs {
			stops = append(stops, s)
		}
		sort.Slice(stops, func(i, j int) bool { return stops[i].StopSequence < stops[j].StopSequence })
		b, err := json.Marshal(stops)
		if err != nil {
			continue
		}
		out[trip] = string(b)
	}
	return out
}
