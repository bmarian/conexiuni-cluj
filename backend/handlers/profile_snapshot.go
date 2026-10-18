package handlers

import (
	"conexiuni-cluj/calendar"
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
}

func (city *City) resetProfileSnapshots() {
	city.profileSnapshots.Lock()
	defer city.profileSnapshots.Unlock()
	city.profileSnapshots.segments = nil
	city.profileSnapshots.delays = nil
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
