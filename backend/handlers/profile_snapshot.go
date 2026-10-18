package handlers

import (
	"conexiuni-cluj/calendar"
	"database/sql"
	"encoding/json"
	"time"
)

const profileSnapshotPeriod = 15 * time.Minute

type profileSnapshotKey struct {
	routeID     int
	directionID int
	dayType     string
}

type scheduledOffsetsKey struct {
	dayType string
	bucket  int
}

type scheduleDelayRow struct {
	bucket int
	delay  float64
}

var profileClock = time.Now

func profilePeriod(now time.Time) int64 {
	return now.Unix() / int64(profileSnapshotPeriod/time.Second)
}

func ProfileSnapshotPeriod() int64 { return profilePeriod(profileClock()) }

func (city *City) profileSnapshotsFor(period int64) {
	if city.profileSnapshots.period == period && city.profileSnapshots.segments != nil {
		return
	}
	city.profileSnapshots.period = period
	city.profileSnapshots.segments = make(map[profileSnapshotKey][]segmentProfileRow)
	city.profileSnapshots.delays = make(map[profileSnapshotKey][]scheduleDelayRow)
	city.profileSnapshots.scheduled = make(map[string]map[scheduledOffsetsKey][]float64)
}

func (city *City) resetProfileSnapshots() {
	city.profileSnapshots.Lock()
	defer city.profileSnapshots.Unlock()
	city.profileSnapshots.segments = nil
	city.profileSnapshots.delays = nil
	city.profileSnapshots.scheduled = nil
}

func (city *City) snapshotSegmentProfiles(routeID, directionID int, dayType string, bucket int) map[stopPair]segmentProfileEstimate {
	key := profileSnapshotKey{routeID, directionID, ""}
	city.profileSnapshots.Lock()
	city.profileSnapshotsFor(ProfileSnapshotPeriod())
	rows, ok := city.profileSnapshots.segments[key]
	city.profileSnapshots.Unlock()
	if !ok {
		loaded, err := city.loadSegmentProfileRows(routeID, directionID)
		if err != nil {
			city.log("stop_times: segment profiles route=%d direction=%d: %v", routeID, directionID, err)
			return map[stopPair]segmentProfileEstimate{}
		}
		rows = loaded
		city.profileSnapshots.Lock()
		city.profileSnapshotsFor(ProfileSnapshotPeriod())
		city.profileSnapshots.segments[key] = rows
		city.profileSnapshots.Unlock()
	}
	return selectSegmentProfiles(rows, dayType, bucket)
}

func (city *City) snapshotScheduleDelay(routeID, directionID int, dayType string, bucket int) (float64, bool) {
	if delay, ok := selectScheduleDelay(city.snapshotScheduleDelayRows(routeID, directionID, dayType), bucket); ok {
		return delay, true
	}
	for _, sibling := range calendar.ProfileDays {
		if calendar.Siblings(sibling, dayType) {
			return selectScheduleDelay(city.snapshotScheduleDelayRows(routeID, directionID, sibling), bucket)
		}
	}
	return 0, false
}

func (city *City) snapshotScheduleDelayRows(routeID, directionID int, dayType string) []scheduleDelayRow {
	key := profileSnapshotKey{routeID, directionID, dayType}
	city.profileSnapshots.Lock()
	city.profileSnapshotsFor(ProfileSnapshotPeriod())
	rows, ok := city.profileSnapshots.delays[key]
	city.profileSnapshots.Unlock()
	if !ok {
		loaded, err := city.loadScheduleDelayRows(routeID, directionID, dayType)
		if err != nil {
			city.log("schedule delay: route=%d direction=%d: %v", routeID, directionID, err)
			return nil
		}
		rows = loaded
		city.profileSnapshots.Lock()
		city.profileSnapshotsFor(ProfileSnapshotPeriod())
		city.profileSnapshots.delays[key] = rows
		city.profileSnapshots.Unlock()
	}
	return rows
}

func (city *City) snapshotScheduledOffsets(tripID, scheduleDay string, bucket int) []float64 {
	city.profileSnapshots.Lock()
	city.profileSnapshotsFor(ProfileSnapshotPeriod())
	byKey, ok := city.profileSnapshots.scheduled[tripID]
	city.profileSnapshots.Unlock()
	if !ok {
		loaded, err := city.loadScheduledOffsets(tripID)
		if err != nil {
			city.log("stop_times: scheduled offsets %s: %v", tripID, err)
			return nil
		}
		byKey = loaded
		city.profileSnapshots.Lock()
		city.profileSnapshotsFor(ProfileSnapshotPeriod())
		city.profileSnapshots.scheduled[tripID] = byKey
		city.profileSnapshots.Unlock()
	}
	return byKey[scheduledOffsetsKey{scheduleDay, bucket}]
}

func (city *City) loadScheduledOffsets(tripID string) (map[scheduledOffsetsKey][]float64, error) {
	out := map[scheduledOffsetsKey][]float64{}
	_, err := queryRows(city.DB, `SELECT day_type, bucket_start_min, offsets FROM scheduled_offsets WHERE trip_id = ?`, []any{tripID},
		func(rows *sql.Rows) (struct{}, error) {
			var key scheduledOffsetsKey
			var offsets string
			if err := rows.Scan(&key.dayType, &key.bucket, &offsets); err != nil {
				return struct{}{}, err
			}
			var seconds []float64
			if err := json.Unmarshal([]byte(offsets), &seconds); err != nil {
				return struct{}{}, err
			}
			out[key] = seconds
			return struct{}{}, nil
		})
	return out, err
}
