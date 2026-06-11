package ctp_cj

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type TimetableEntry struct {
	DepartureIn  string
	DepartureOut string
}

// Frequency describes a headway-based direction (e.g. line M26), where CTP lists
// no individual departures, only a service window and a "every N-M min" interval.
type Frequency struct {
	Start      string
	End        string
	MinMinutes int
	MaxMinutes int
}

type ParsedTimetable struct {
	RouteLongName string
	ServiceName   string
	ServiceStart  string
	InStopName    string
	OutStopName   string
	Entries       []TimetableEntry
	InFrequency   *Frequency
	OutFrequency  *Frequency
}

// Accept HH:MM and annotated values like 07:20*.
var timeCell = regexp.MustCompile(`^\s*\d{1,2}:\d{2}\S*\s*$`)

// Headway cells, spread across two rows in one column: a service window
// ("05:10-22:40") and an interval ("10-20min" or "15min").
var (
	freqWindowCell   = regexp.MustCompile(`^(\d{1,2}:\d{2})-(\d{1,2}:\d{2})$`)
	freqIntervalCell = regexp.MustCompile(`^(\d{1,2})(?:\s*-\s*(\d{1,2}))?\s*min$`)
)

func isTimeCell(s string) bool {
	return s == "" || timeCell.MatchString(s)
}

// applyFreqCell folds a window or interval cell into freq (allocated on first
// use) and reports whether s was a frequency cell. The two halves arrive on
// separate rows, so they accumulate into one struct.
func applyFreqCell(freq **Frequency, s string) bool {
	if m := freqWindowCell.FindStringSubmatch(s); m != nil {
		if *freq == nil {
			*freq = &Frequency{}
		}
		(*freq).Start, (*freq).End = m[1], m[2]
		return true
	}
	if m := freqIntervalCell.FindStringSubmatch(s); m != nil {
		if *freq == nil {
			*freq = &Frequency{}
		}
		minv, _ := strconv.Atoi(m[1])
		maxv := minv
		if m[2] != "" {
			maxv, _ = strconv.Atoi(m[2])
		}
		(*freq).MinMinutes, (*freq).MaxMinutes = minv, maxv
		return true
	}
	return false
}

func ParseTimetableCSV(data []byte) (*ParsedTimetable, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	scanner := bufio.NewScanner(bytes.NewReader(data))

	t := &ParsedTimetable{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

		switch left {
		case "route_long_name":
			t.RouteLongName = right
			continue
		case "service_name":
			t.ServiceName = right
			continue
		case "service_start":
			t.ServiceStart = right
			continue
		case "in_stop_name":
			t.InStopName = right
			continue
		case "out_stop_name":
			t.OutStopName = right
			continue
		}

		// A headway column carries no individual departures, only the window /
		// interval cells. Lift them into the frequency descriptor and blank the
		// cell so the opposite column's real time on that row is still kept.
		if applyFreqCell(&t.InFrequency, left) {
			left = ""
		}
		if applyFreqCell(&t.OutFrequency, right) {
			right = ""
		}

		if !isTimeCell(left) || !isTimeCell(right) {
			continue
		}
		t.Entries = append(t.Entries, TimetableEntry{
			DepartureIn:  left,
			DepartureOut: right,
		})
	}

	normalizeEntries(t.Entries)
	return t, scanner.Err()
}

// normalizeEntries rewrites post-midnight times to GTFS 24+ hour notation
// (e.g. "00:30" after "23:30" becomes "24:30") by detecting backward transitions
// in each direction's sequence independently.
func normalizeEntries(entries []TimetableEntry) {
	prevIn, prevOut := -1, -1
	offIn, offOut := 0, 0
	for i := range entries {
		entries[i].DepartureIn = normalizeTime(entries[i].DepartureIn, &prevIn, &offIn)
		entries[i].DepartureOut = normalizeTime(entries[i].DepartureOut, &prevOut, &offOut)
	}
}

func normalizeTime(s string, prev *int, offset *int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Split "HH:MM" from any trailing annotation (e.g. "*").
	colon := strings.Index(s, ":")
	if colon < 0 {
		return s
	}
	end := colon + 1
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	timeStr, suffix := s[:end], s[end:]

	parts := strings.SplitN(timeStr, ":", 2)
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return s
	}

	cur := h*60 + m
	if *prev >= 0 && cur < *prev {
		*offset += 1440
	}
	*prev = cur

	if *offset == 0 {
		return s
	}
	total := cur + *offset
	return fmt.Sprintf("%02d:%02d%s", total/60, total%60, suffix)
}
