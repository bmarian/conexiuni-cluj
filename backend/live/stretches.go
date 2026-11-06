package live

import (
	"conexiuni-cluj/models"
	"time"
)

// AllDay is the Hour of a profile over the whole day.
const AllDay = -1

const MinProfileSamples = minSegmentProfileSamples

type Stretch struct {
	FromStopID int
	ToStopID   int
	Points     []models.Shape
}

type SegmentProfile struct {
	FromStopID  int
	ToStopID    int
	DayType     string
	Hour        int
	SampleCount int
	MedianSec   float64
}

// Stretches are a pattern's stop-to-stop pieces in stop order, between the ends the passengers see.
func (eng *Engine) Stretches(tripID string) ([]Stretch, bool) {
	tracker, ok := eng.getTripSegmentTracker(tripID)
	if !ok {
		return nil, false
	}
	first, last := visibleEnds(len(tracker.Stops), func(pos int) bool { return eng.StopHidden(tracker.Stops[pos].StopID) })
	out := make([]Stretch, 0, len(tracker.Stops))
	for i := first; i < last; i++ {
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

type StretchRun struct {
	FromStopID  int
	ToStopID    int
	DayType     string
	Hour        int
	DurationSec float64
	ObservedAt  int64
}

// Oldest first.
func (eng *Engine) EachStretchRun(since time.Time, fn func(StretchRun)) error {
	rows, err := eng.DB.Query(`
		SELECT from_stop_id, to_stop_id, day_type, bucket_start_min, duration_sec, observed_at
		FROM segment_travel_time_samples INDEXED BY idx_segment_samples_observed_at
		WHERE observed_at >= ?
		ORDER BY observed_at`, since.Unix())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r StretchRun
		var bucket int
		if err := rows.Scan(&r.FromStopID, &r.ToStopID, &r.DayType, &bucket, &r.DurationSec, &r.ObservedAt); err != nil {
			return err
		}
		r.Hour = bucket / 60
		fn(r)
	}
	return rows.Err()
}
