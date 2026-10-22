package gtfs

import (
	"conexiuni-cluj/models"
	"fmt"
	"strconv"
	"strings"
)

// A frequencies.txt row: runs leave every everyMin from startMin until before endMin.
type headway struct {
	startMin, endMin, everyMin int
}

// expandFrequencies replaces every trip frequencies.txt names by one run per departure, its stop times shifted, and returns each run's window.
func (f *feed) expandFrequencies() map[string]headway {
	freq, ok := f.files["frequencies"]
	if !ok {
		return nil
	}
	out := map[string]headway{}
	templates := map[string]bool{}
	for _, row := range freq.rows {
		tripID := freq.get(row, "trip_id")
		info, known := f.trips[tripID]
		run := f.stops[tripID]
		start, end := secondsOrMissing(freq.get(row, "start_time")), secondsOrMissing(freq.get(row, "end_time"))
		every, err := strconv.Atoi(strings.TrimSpace(freq.get(row, "headway_secs")))
		if !known || len(run) == 0 || startMinutes(run) < 0 || start < 0 || end <= start || err != nil || every <= 0 {
			continue
		}
		templates[tripID] = true
		window := headway{startMin: start / 60, endMin: end / 60, everyMin: (every + 30) / 60}
		for at := start; at < end; at += every {
			shift := at/60 - startMinutes(run)
			shifted := make([]stopTime, len(run))
			for i, st := range run {
				shifted[i] = st
				if st.arrival >= 0 {
					shifted[i].arrival += shift
				}
				if st.departure >= 0 {
					shifted[i].departure += shift
				}
			}
			id := fmt.Sprintf("%s@%02d:%02d", tripID, at/3600, at/60%60)
			f.trips[id], f.stops[id], out[id] = info, shifted, window
		}
	}
	for tripID := range templates {
		delete(f.trips, tripID)
		delete(f.stops, tripID)
	}
	return out
}

// Seconds of a GTFS time, which may pass 24:00; -1 when missing.
func secondsOrMissing(hhmmss string) int {
	minutes := minutesOrMissing(hhmmss)
	if minutes < 0 {
		return -1
	}
	seconds := 0
	if parts := strings.Split(hhmmss, ":"); len(parts) > 2 {
		seconds, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	}
	return minutes*60 + seconds
}

func frequencyOf(windows []headway) *models.Frequency {
	if len(windows) == 0 {
		return nil
	}
	first, last := windows[0], windows[0]
	f := &models.Frequency{MinMinutes: first.everyMin, MaxMinutes: first.everyMin}
	for _, w := range windows {
		f.MinMinutes, f.MaxMinutes = min(f.MinMinutes, w.everyMin), max(f.MaxMinutes, w.everyMin)
		if w.startMin < first.startMin {
			first = w
		}
		if w.endMin > last.endMin {
			last = w
		}
	}
	f.Start = fmt.Sprintf("%02d:%02d", first.startMin/60, first.startMin%60)
	f.End = fmt.Sprintf("%02d:%02d", last.endMin/60, last.endMin%60)
	return f
}
