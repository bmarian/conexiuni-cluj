package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

// ArchiveVersions keeps today's timetables and stop lists in the archive, where they changed.
func (d *Data) ArchiveVersions(now time.Time) (int, error) {
	timetables, err := database.QueryRows(d.db, `
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
	stored, err := d.store.ArchiveTimetables(timetables, now)
	if err != nil {
		return stored, err
	}
	stopTimes, err := d.StopTimes(StopTimeFilter{})
	if err != nil {
		return stored, err
	}
	trips, err := d.store.ArchiveTripStops(tripStopsJSON(stopTimes), now)
	return stored + trips, err
}

func (d *Data) archiveTimetable(t *models.Timetable, weekdays, saturday, sunday string) {
	_, err := d.store.ArchiveTimetables([]database.TimetableVersion{{
		RouteShortName: t.RouteShortName,
		RouteLongName:  t.RouteLongName,
		InStopName:     t.InStopName,
		OutStopName:    t.OutStopName,
		Weekdays:       weekdays,
		Saturday:       saturday,
		Sunday:         sunday,
	}}, time.Now())
	if err != nil {
		d.store.Logf("archive: timetable %s failed: %v", t.RouteShortName, err)
	}
}

func (d *Data) archiveStopTimes(stopTimes []models.StopTime) {
	if _, err := d.store.ArchiveTripStops(tripStopsJSON(stopTimes), time.Now()); err != nil {
		d.store.Logf("archive: stop times failed: %v", err)
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
