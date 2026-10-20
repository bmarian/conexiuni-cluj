package live

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"database/sql"
	"encoding/json"
	"sync"
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

func (eng *Engine) profileSnapshotsFor(period int64) {
	if eng.profileSnapshots.period == period && eng.profileSnapshots.segments != nil {
		return
	}
	eng.profileSnapshots.period = period
	eng.profileSnapshots.segments = make(map[profileSnapshotKey][]segmentProfileRow)
	eng.profileSnapshots.delays = make(map[profileSnapshotKey][]scheduleDelayRow)
	eng.profileSnapshots.scheduled = make(map[string]map[scheduledOffsetsKey][]float64)
}

func (eng *Engine) resetProfileSnapshots() {
	eng.profileSnapshots.Lock()
	defer eng.profileSnapshots.Unlock()
	eng.profileSnapshots.segments = nil
	eng.profileSnapshots.delays = nil
	eng.profileSnapshots.scheduled = nil
}

func (eng *Engine) snapshotSegmentProfiles(routeID, directionID int, dayType string, bucket int) map[stopPair]segmentProfileEstimate {
	key := profileSnapshotKey{routeID, directionID, ""}
	eng.profileSnapshots.Lock()
	eng.profileSnapshotsFor(ProfileSnapshotPeriod())
	rows, ok := eng.profileSnapshots.segments[key]
	eng.profileSnapshots.Unlock()
	if !ok {
		loaded, err := eng.loadSegmentProfileRows(routeID, directionID)
		if err != nil {
			eng.log("stop_times: segment profiles route=%d direction=%d: %v", routeID, directionID, err)
			return map[stopPair]segmentProfileEstimate{}
		}
		rows = loaded
		eng.profileSnapshots.Lock()
		eng.profileSnapshotsFor(ProfileSnapshotPeriod())
		eng.profileSnapshots.segments[key] = rows
		eng.profileSnapshots.Unlock()
	}
	return selectSegmentProfiles(rows, dayType, bucket)
}

func (eng *Engine) snapshotScheduleDelay(routeID, directionID int, dayType string, bucket int) (float64, bool) {
	if delay, ok := selectScheduleDelay(eng.snapshotScheduleDelayRows(routeID, directionID, dayType), bucket); ok {
		return delay, true
	}
	for _, sibling := range calendar.ProfileDays {
		if calendar.Siblings(sibling, dayType) {
			return selectScheduleDelay(eng.snapshotScheduleDelayRows(routeID, directionID, sibling), bucket)
		}
	}
	return 0, false
}

func (eng *Engine) snapshotScheduleDelayRows(routeID, directionID int, dayType string) []scheduleDelayRow {
	key := profileSnapshotKey{routeID, directionID, dayType}
	eng.profileSnapshots.Lock()
	eng.profileSnapshotsFor(ProfileSnapshotPeriod())
	rows, ok := eng.profileSnapshots.delays[key]
	eng.profileSnapshots.Unlock()
	if !ok {
		loaded, err := eng.loadScheduleDelayRows(routeID, directionID, dayType)
		if err != nil {
			eng.log("schedule delay: route=%d direction=%d: %v", routeID, directionID, err)
			return nil
		}
		rows = loaded
		eng.profileSnapshots.Lock()
		eng.profileSnapshotsFor(ProfileSnapshotPeriod())
		eng.profileSnapshots.delays[key] = rows
		eng.profileSnapshots.Unlock()
	}
	return rows
}

func (eng *Engine) snapshotScheduledOffsets(tripID, scheduleDay string, bucket int) []float64 {
	eng.profileSnapshots.Lock()
	eng.profileSnapshotsFor(ProfileSnapshotPeriod())
	byKey, ok := eng.profileSnapshots.scheduled[tripID]
	eng.profileSnapshots.Unlock()
	if !ok {
		loaded, err := eng.loadScheduledOffsets(tripID)
		if err != nil {
			eng.log("stop_times: scheduled offsets %s: %v", tripID, err)
			return nil
		}
		byKey = loaded
		eng.profileSnapshots.Lock()
		eng.profileSnapshotsFor(ProfileSnapshotPeriod())
		eng.profileSnapshots.scheduled[tripID] = byKey
		eng.profileSnapshots.Unlock()
	}
	return byKey[scheduledOffsetsKey{scheduleDay, bucket}]
}

func (eng *Engine) loadScheduledOffsets(tripID string) (map[scheduledOffsetsKey][]float64, error) {
	out := map[scheduledOffsetsKey][]float64{}
	_, err := database.QueryRows(eng.DB, `SELECT day_type, bucket_start_min, offsets FROM scheduled_offsets WHERE trip_id = ?`, []any{tripID},
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

type profileSnapshotsState struct {
	sync.Mutex
	period    int64
	segments  map[profileSnapshotKey][]segmentProfileRow
	delays    map[profileSnapshotKey][]scheduleDelayRow
	scheduled map[string]map[scheduledOffsetsKey][]float64
}
