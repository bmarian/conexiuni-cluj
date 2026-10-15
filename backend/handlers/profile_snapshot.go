package handlers

import (
	"conexiuni-cluj/calendar"
	"log"
	"sync"
	"time"
)

const profileSnapshotPeriod = 15 * time.Minute

type profileSnapshotKey struct {
	routeID     int
	directionID int
	dayType     string
}

type scheduleDelayRow struct {
	bucket int
	delay  float64
}

var profileSnapshots = struct {
	sync.Mutex
	period   int64
	segments map[profileSnapshotKey][]segmentProfileRow
	delays   map[profileSnapshotKey][]scheduleDelayRow
}{}

var profileClock = time.Now

func profilePeriod(now time.Time) int64 {
	return now.Unix() / int64(profileSnapshotPeriod/time.Second)
}

func ProfileSnapshotPeriod() int64 { return profilePeriod(profileClock()) }

func profileSnapshotsFor(period int64) {
	if profileSnapshots.period == period && profileSnapshots.segments != nil {
		return
	}
	profileSnapshots.period = period
	profileSnapshots.segments = make(map[profileSnapshotKey][]segmentProfileRow)
	profileSnapshots.delays = make(map[profileSnapshotKey][]scheduleDelayRow)
}

func resetProfileSnapshots() {
	profileSnapshots.Lock()
	defer profileSnapshots.Unlock()
	profileSnapshots.segments = nil
	profileSnapshots.delays = nil
}

func snapshotSegmentProfiles(routeID, directionID int, dayType string, bucket int) map[stopPair]segmentProfileEstimate {
	key := profileSnapshotKey{routeID, directionID, ""}
	profileSnapshots.Lock()
	profileSnapshotsFor(ProfileSnapshotPeriod())
	rows, ok := profileSnapshots.segments[key]
	profileSnapshots.Unlock()
	if !ok {
		loaded, err := loadSegmentProfileRows(routeID, directionID)
		if err != nil {
			log.Printf("stop_times: segment profiles route=%d direction=%d: %v", routeID, directionID, err)
			return map[stopPair]segmentProfileEstimate{}
		}
		rows = loaded
		profileSnapshots.Lock()
		profileSnapshotsFor(ProfileSnapshotPeriod())
		profileSnapshots.segments[key] = rows
		profileSnapshots.Unlock()
	}
	return selectSegmentProfiles(rows, dayType, bucket)
}

func snapshotScheduleDelay(routeID, directionID int, dayType string, bucket int) (float64, bool) {
	if delay, ok := selectScheduleDelay(snapshotScheduleDelayRows(routeID, directionID, dayType), bucket); ok {
		return delay, true
	}
	for _, sibling := range calendar.ProfileDays {
		if calendar.Siblings(sibling, dayType) {
			return selectScheduleDelay(snapshotScheduleDelayRows(routeID, directionID, sibling), bucket)
		}
	}
	return 0, false
}

func snapshotScheduleDelayRows(routeID, directionID int, dayType string) []scheduleDelayRow {
	key := profileSnapshotKey{routeID, directionID, dayType}
	profileSnapshots.Lock()
	profileSnapshotsFor(ProfileSnapshotPeriod())
	rows, ok := profileSnapshots.delays[key]
	profileSnapshots.Unlock()
	if !ok {
		loaded, err := loadScheduleDelayRows(routeID, directionID, dayType)
		if err != nil {
			log.Printf("schedule delay: route=%d direction=%d: %v", routeID, directionID, err)
			return nil
		}
		rows = loaded
		profileSnapshots.Lock()
		profileSnapshotsFor(ProfileSnapshotPeriod())
		profileSnapshots.delays[key] = rows
		profileSnapshots.Unlock()
	}
	return rows
}
