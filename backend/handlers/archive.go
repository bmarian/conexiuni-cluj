package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"encoding/json"
	"log"
	"sort"
	"strings"
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

func (city *City) StartArchiver(loc *time.Location) {
	if city.Archive == nil {
		return
	}
	go func() {
		time.Sleep(archiveStartDelay)
		for {
			city.checkObservedSchedule(time.Now())
			city.runArchive(time.Now())
			city.tuneVacationWindow(time.Now())
			city.tuneSeedFill(time.Now())
			city.restoreSeededWindows(time.Now())
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

func (city *City) runArchive(now time.Time) {
	started := time.Now()
	run := archiveRun{At: now}
	var errs []string
	copied, err := city.CopyToArchive(now.Add(-archiveSettle))
	for _, n := range copied {
		run.Copied += n
	}
	if err != nil {
		errs = append(errs, "copy: "+err.Error())
	}
	if run.Versions, err = city.snapshotArchiveVersions(now); err != nil {
		errs = append(errs, "versions: "+err.Error())
	}
	if run.Pruned, err = city.PruneArchive(now.Add(-archiveRetention)); err != nil {
		errs = append(errs, "prune: "+err.Error())
	}
	if run.Summary, err = city.SummarizeArchive(); err != nil {
		errs = append(errs, "summary: "+err.Error())
	}
	run.Err = strings.Join(errs, "; ")
	run.Took = time.Since(started)
	if run.Err != "" {
		log.Printf("archive failed: %s", run.Err)
	} else {
		log.Printf("archive: copied %d rows, %d new timetable or stop versions, pruned %d in %s", run.Copied, run.Versions, run.Pruned, run.Took.Round(time.Millisecond))
	}
	city.lastArchive.Lock()
	city.lastArchive.run = &run
	city.lastArchive.Unlock()
}

func (city *City) lastArchiveRun() *archiveRun {
	city.lastArchive.Lock()
	defer city.lastArchive.Unlock()
	return city.lastArchive.run
}

func (city *City) snapshotArchiveVersions(now time.Time) (int, error) {
	timetables, err := queryRows(city.DB, `
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
	stored, err := city.ArchiveTimetables(timetables, now)
	if err != nil {
		return stored, err
	}
	stopTimes, err := city.getStopTimesFromDB(StopTimeFilter{})
	if err != nil {
		return stored, err
	}
	trips, err := city.ArchiveTripStops(tripStopsJSON(stopTimes), now)
	return stored + trips, err
}

func (city *City) archiveTimetable(t *models.Timetable, weekdays, saturday, sunday string) {
	_, err := city.ArchiveTimetables([]database.TimetableVersion{{
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

func (city *City) archiveStopTimes(stopTimes []models.StopTime) {
	if _, err := city.ArchiveTripStops(tripStopsJSON(stopTimes), time.Now()); err != nil {
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
