package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"time"
)

// AllDay is the Hour of a profile over the whole day.
const AllDay = -1

// MinProfileSamples is how many timings make a profile worth reading.
const MinProfileSamples = minSegmentProfileSamples

// Stretch is one stop-to-stop piece of a pattern with the shape points it runs along.
type Stretch struct {
	FromStopID int
	ToStopID   int
	Points     []models.Shape
}

// SegmentProfile is the learned time of a stretch for one hour of a day type.
type SegmentProfile struct {
	FromStopID  int
	ToStopID    int
	DayType     string
	Hour        int
	SampleCount int
	MedianSec   float64
}

// Stretches are a pattern's stop-to-stop pieces, in stop order.
func (eng *Engine) Stretches(tripID string) ([]Stretch, bool) {
	tracker, ok := eng.getTripSegmentTracker(tripID)
	if !ok {
		return nil, false
	}
	out := make([]Stretch, 0, len(tracker.Stops))
	for i := 0; i+1 < len(tracker.Stops); i++ {
		from, to := tracker.Stops[i], tracker.Stops[i+1]
		if to.ShapeIdx <= from.ShapeIdx {
			continue
		}
		out = append(out, Stretch{FromStopID: from.StopID, ToStopID: to.StopID, Points: tracker.Shape[from.ShapeIdx : to.ShapeIdx+1]})
	}
	return out, true
}

// SegmentProfiles are a direction's hourly stretch times with enough samples to trust, and its all-day ones as Hour AllDay.
func (eng *Engine) SegmentProfiles(routeID, directionID int) ([]SegmentProfile, error) {
	rows, err := eng.loadSegmentProfileRows(routeID, directionID)
	if err != nil {
		return nil, err
	}
	out := make([]SegmentProfile, 0, len(rows))
	for _, r := range rows {
		hour := AllDay
		if r.bucket != allDaySegmentBucket {
			hour = r.bucket / 60
		}
		out = append(out, SegmentProfile{
			FromStopID: r.pair.FromStopID, ToStopID: r.pair.ToStopID, DayType: r.dayType,
			Hour: hour, SampleCount: r.sampleCount, MedianSec: r.medianSec,
		})
	}
	return out, nil
}

// RecentStretch is what the buses since a moment did over a stretch.
type RecentStretch struct {
	RouteID     int
	DirectionID int
	FromStopID  int
	ToStopID    int
	Samples     int
	MeanSec     float64
}

// RecentStretchTimes are every stretch's timings since a moment, by stretch.
func (eng *Engine) RecentStretchTimes(since time.Time) ([]RecentStretch, error) {
	return database.QueryRows(eng.DB, `
		SELECT route_id, direction_id, from_stop_id, to_stop_id, COUNT(*), AVG(duration_sec)
		FROM segment_travel_time_samples INDEXED BY idx_segment_samples_observed_at
		WHERE observed_at >= ?
		GROUP BY route_id, direction_id, from_stop_id, to_stop_id`,
		[]any{since.Unix()},
		func(rows *sql.Rows) (RecentStretch, error) {
			var r RecentStretch
			err := rows.Scan(&r.RouteID, &r.DirectionID, &r.FromStopID, &r.ToStopID, &r.Samples, &r.MeanSec)
			return r, err
		})
}

func (eng *Engine) ProfileDay(t time.Time) string { return eng.profileDay(t) }
