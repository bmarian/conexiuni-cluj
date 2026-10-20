package live

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

const (
	shiftBaselineDays = 10
	shiftSpanDays     = 10
	shiftMorningFrom  = 6 * 60
	shiftMorningTo    = 10 * 60
)

type shiftKey struct {
	route  replayRoute
	pair   stopPair
	bucket int16
}

func (eng *Engine) RunShift(date time.Time, w io.Writer) error {
	if eng.Archive == nil {
		return fmt.Errorf("the archive database is not open")
	}
	cal := eng.currentCalendar()
	day := eng.localMidnight(date)
	dayType := cal.ProfileDay(day)
	days := newReplayDays(eng)

	before := eng.daysOfType(cal, day, dayType, shiftSpanDays)
	baselineDays, err := days.days(before[:min(shiftBaselineDays, len(before))])
	if err != nil {
		return err
	}
	groups := map[shiftKey][]float64{}
	for _, samples := range baselineDays {
		for _, s := range samples {
			d := float64(s.durationS)
			groups[shiftKey{s.route, s.pair, s.bucket}] = append(groups[shiftKey{s.route, s.pair, s.bucket}], d)
			groups[shiftKey{s.route, s.pair, allDaySegmentBucket}] = append(groups[shiftKey{s.route, s.pair, allDaySegmentBucket}], d)
		}
	}
	baseline := map[shiftKey]float64{}
	for k, durations := range groups {
		if len(durations) >= minSegmentProfileSamples {
			baseline[k] = median(durations)
		}
	}
	ratio := func(s replaySample) (float64, bool) {
		b, ok := baseline[shiftKey{s.route, s.pair, s.bucket}]
		if !ok {
			b, ok = baseline[shiftKey{s.route, s.pair, allDaySegmentBucket}]
		}
		if !ok || b <= 0 {
			return 0, false
		}
		return float64(s.durationS) / b, true
	}

	var rows []time.Time
	for i := len(before) - 1; i >= 0; i-- {
		rows = append(rows, before[i])
	}
	rows = append(rows, day)
	for i, found := 1, 0; i <= replaySearchDays && found < shiftSpanDays; i++ {
		if d := day.AddDate(0, 0, i); cal.ProfileDay(d) == dayType {
			rows = append(rows, d)
			found++
		}
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	_, _ = fmt.Fprintln(tw, "Day\tAll day\t06:00–10:00\tTimes used\t\t")
	for _, d := range rows {
		samples, err := days.day(d)
		if err != nil {
			return err
		}
		var all, morning []float64
		for _, s := range samples {
			r, ok := ratio(s)
			if !ok {
				continue
			}
			all = append(all, r)
			if int(s.bucket) >= shiftMorningFrom && int(s.bucket) < shiftMorningTo {
				morning = append(morning, r)
			}
		}
		mark := ""
		if d.Equal(day) {
			mark = "<- " + day.Format("2006-01-02")
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t\n", d.Format("Mon 2 Jan"), shiftLevel(all), shiftLevel(morning), len(all), mark)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "\nKind of day: %s. Each value is the median of travel time ÷ the median of the %d days of this kind before %s, for the same stop pair and hour, or the whole day when that hour has fewer than %d samples.\nThe days before %s are part of that baseline, so they sit near 1.00. A lasting step from %s on is a shift.\n",
		dayType, len(baselineDays), day.Format("2006-01-02"), minSegmentProfileSamples, day.Format("2006-01-02"), day.Format("2006-01-02"))
	return err
}

func shiftLevel(ratios []float64) string {
	if len(ratios) == 0 {
		return "–"
	}
	return fmt.Sprintf("%.2f", median(ratios))
}
