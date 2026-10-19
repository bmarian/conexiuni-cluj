package handlers

import (
	"conexiuni-cluj/calendar"
	"fmt"
	"io"
	"math"
	"sort"
	"text/tabwriter"
	"time"
)

const (
	replayErrorCapSec = 600.0
	replaySearchDays  = 400
	legacyWindowDays  = 14
	replayAnyDay      = "any"
)

type replaySample struct {
	route     replayRoute
	pair      stopPair
	bucket    int16
	durationS float32
}

type replayRoute struct {
	route, direction int32
}

type replayDays struct {
	city   *City
	loaded map[string][]replaySample
}

func newReplayDays(city *City) *replayDays {
	return &replayDays{city: city, loaded: map[string][]replaySample{}}
}

func (r *replayDays) day(d time.Time) ([]replaySample, error) {
	city := r.city
	start := city.localMidnight(d)
	key := start.Format("2006-01-02")
	if samples, ok := r.loaded[key]; ok {
		return samples, nil
	}
	var samples []replaySample
	err := city.ArchivedSegmentSamples(start.Unix(), start.AddDate(0, 0, 1).Unix(), func(route, direction, from, to, bucket int, duration float64) {
		samples = append(samples, replaySample{
			route: replayRoute{int32(route), int32(direction)}, pair: stopPair{FromStopID: from, ToStopID: to},
			bucket: int16(bucket), durationS: float32(duration),
		})
	})
	if err != nil {
		return nil, err
	}
	r.loaded[key] = samples
	return samples, nil
}

func (r *replayDays) days(dates []time.Time) ([][]replaySample, error) {
	out := make([][]replaySample, 0, len(dates))
	for _, d := range dates {
		samples, err := r.day(d)
		if err != nil {
			return nil, err
		}
		out = append(out, samples)
	}
	return out, nil
}

func (city *City) daysOfType(cal *calendar.Calendar, before time.Time, dayType string, n int) []time.Time {
	var out []time.Time
	day := city.localMidnight(before)
	for i := 1; i <= replaySearchDays && len(out) < n; i++ {
		d := day.AddDate(0, 0, -i)
		if cal.ProfileDay(d) == dayType {
			out = append(out, d)
		}
	}
	return out
}

func plainDay(t time.Time) string {
	switch t.Weekday() {
	case time.Saturday:
		return calendar.Saturday
	case time.Sunday:
		return calendar.Sunday
	}
	return calendar.Weekday
}

func replayProfiles(days [][]replaySample, dayType string) map[replayRoute][]segmentProfileRow {
	type groupKey struct {
		route  replayRoute
		pair   stopPair
		bucket int
	}
	groups := map[groupKey][]float64{}
	for _, samples := range days {
		for _, s := range samples {
			d := float64(s.durationS)
			k := groupKey{s.route, s.pair, int(s.bucket)}
			groups[k] = append(groups[k], d)
			k.bucket = allDaySegmentBucket
			groups[k] = append(groups[k], d)
		}
	}
	out := map[replayRoute][]segmentProfileRow{}
	for k, durations := range groups {
		if len(durations) < minSegmentProfileSamples {
			continue
		}
		sort.Float64s(durations)
		out[k.route] = append(out[k.route], segmentProfileRow{
			pair: k.pair, dayType: dayType, bucket: k.bucket, sampleCount: len(durations), medianSec: nearestRank(durations, 0.5),
		})
	}
	return out
}

func mergeProfileRows(sets ...map[replayRoute][]segmentProfileRow) map[replayRoute][]segmentProfileRow {
	out := map[replayRoute][]segmentProfileRow{}
	for _, set := range sets {
		for route, rows := range set {
			out[route] = append(out[route], rows...)
		}
	}
	return out
}

func replayPredictor(rows map[replayRoute][]segmentProfileRow, dayType string) func(replaySample) (float64, bool) {
	type cacheKey struct {
		route  replayRoute
		bucket int16
	}
	cache := map[cacheKey]map[stopPair]segmentProfileEstimate{}
	return func(s replaySample) (float64, bool) {
		k := cacheKey{s.route, s.bucket}
		selected, ok := cache[k]
		if !ok {
			selected = selectSegmentProfiles(rows[s.route], dayType, int(s.bucket))
			cache[k] = selected
		}
		est, ok := selected[s.pair]
		return est.DurationSec, ok && est.DurationSec > 0
	}
}

func replayError(s replaySample, predicted float64) float64 {
	return math.Min(math.Abs(float64(s.durationS)-predicted), replayErrorCapSec)
}

func (city *City) RunReplay(from, to time.Time, w io.Writer) error {
	if city.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	cal := city.currentCalendar()
	days := newReplayDays(city)
	type total struct {
		days, compared int
		ours, legacy   float64
	}
	totals := map[string]*total{}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	_, _ = fmt.Fprintln(tw, "Day\tKind of day\tTimes compared\tCalendar windows\tLast 14 days\tDifference\t")
	for d := city.localMidnight(from); !d.After(city.localMidnight(to)); d = d.AddDate(0, 0, 1) {
		samples, err := days.day(d)
		if err != nil {
			return err
		}
		if len(samples) == 0 {
			continue
		}
		dayType := cal.ProfileDay(d)
		size, _ := city.dayWindowSize(dayType)
		ownDays, err := days.days(city.daysOfType(cal, d, dayType, size))
		if err != nil {
			return err
		}
		var legacyDates []time.Time
		for i := 1; i <= legacyWindowDays; i++ {
			if prev := d.AddDate(0, 0, -i); plainDay(prev) == plainDay(d) {
				legacyDates = append(legacyDates, prev)
			}
		}
		legacyDays, err := days.days(legacyDates)
		if err != nil {
			return err
		}
		ours := replayPredictor(replayProfiles(ownDays, replayAnyDay), replayAnyDay)
		legacy := replayPredictor(replayProfiles(legacyDays, replayAnyDay), replayAnyDay)

		var sumOurs, sumLegacy float64
		compared := 0
		for _, s := range samples {
			a, okA := ours(s)
			b, okB := legacy(s)
			if !okA || !okB {
				continue
			}
			compared++
			sumOurs += replayError(s, a)
			sumLegacy += replayError(s, b)
		}
		if compared == 0 {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t0\t–\t–\t\t\n", d.Format("Mon 2 Jan"), dayType)
			continue
		}
		t := totals[dayType]
		if t == nil {
			t = &total{}
			totals[dayType] = t
		}
		t.days++
		t.compared += compared
		t.ours += sumOurs
		t.legacy += sumLegacy
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%.0f s\t%.0f s\t%s\t\n", d.Format("Mon 2 Jan"), dayType, compared,
			sumOurs/float64(compared), sumLegacy/float64(compared), replayDifference(sumOurs, sumLegacy))
	}
	_, _ = fmt.Fprintln(tw, "\t\t\t\t\t\t")
	for _, dayType := range calendar.ProfileDays {
		t := totals[dayType]
		if t == nil {
			continue
		}
		_, _ = fmt.Fprintf(tw, "%d days\t%s\t%d\t%.0f s\t%.0f s\t%s\t\n", t.days, dayType, t.compared,
			t.ours/float64(t.compared), t.legacy/float64(t.compared), replayDifference(t.ours, t.legacy))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "\nAverage miss of the learned stop-to-stop time, each capped at 10 minutes. Negative differences mean the calendar windows did better.")
	return err
}

func replayDifference(ours, legacy float64) string {
	if legacy == 0 {
		return ""
	}
	return fmt.Sprintf("%+.0f%%", (ours-legacy)/legacy*100)
}
