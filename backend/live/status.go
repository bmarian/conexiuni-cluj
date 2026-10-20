package live

import "time"

// Status is the live side at a glance, for the admin page.
type Status struct {
	Watching     Watching
	BusesTracked int
	LinesLive    int
	Feed         FeedStatus
	Learning     LearnerStatus
	// How often vehicles are polled now and for whom: "viewers" or "learner"; zero when they are not.
	PollInterval time.Duration
	PolledBy     string
	// Nil until the archive first ran.
	Archive         *ArchiveRun
	CalendarProblem string
}

func (eng *Engine) Status(now time.Time) Status {
	s := Status{Watching: eng.hub.Watching(), Feed: eng.currentFeedStatus(), Learning: eng.currentVehicleLearningStatus(), Archive: eng.lastArchiveRun(), CalendarProblem: eng.calendarProblem(now)}
	if table := eng.currentLiveTable(); table != nil {
		lines := make(map[int]struct{})
		for _, vehicles := range table.vehiclesByTrip {
			for _, v := range vehicles {
				s.BusesTracked++
				lines[v.RouteID] = struct{}{}
			}
		}
		s.LinesLive = len(lines)
	}
	if s.Watching.Streams > 0 {
		s.PollInterval, s.PolledBy = eng.hub.CurrentInterval(), "viewers"
	} else if s.Learning.Enabled {
		s.PollInterval, s.PolledBy = s.Learning.Interval, "learner"
	}
	return s
}

// LearningActivity is how many travel times were measured today and in the last hour, and when the delays were last learned.
func (eng *Engine) LearningActivity(now time.Time) (today, lastHour int, delaysAt time.Time, err error) {
	local := now.In(eng.Location)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, eng.Location).Unix()
	var at int64
	err = eng.DB.QueryRow(`
		SELECT (SELECT COUNT(*) FROM segment_travel_time_samples WHERE observed_at >= ?),
		       (SELECT COUNT(*) FROM segment_travel_time_samples WHERE observed_at >= ?),
		       (SELECT COALESCE(MAX(updated_at), 0) FROM schedule_delay_profiles)`,
		midnight, now.Add(-time.Hour).Unix()).Scan(&today, &lastHour, &at)
	if at > 0 {
		delaysAt = time.Unix(at, 0)
	}
	return today, lastHour, delaysAt, err
}
