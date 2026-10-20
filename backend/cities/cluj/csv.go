package cluj

import (
	"bufio"
	"bytes"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"regexp"
	"strconv"
	"strings"
)

// One of CTP's files: a line's departures on one kind of day, from both termini.
type csvDay struct {
	routeLongName, inStopName, outStopName string
	models.DaySchedule
}

// Accept HH:MM and annotated values like 07:20* or *07:20.
var timeCell = regexp.MustCompile(`^\s*\**\d{1,2}:\d{2}\S*\s*$`)

// A window ("05:10-22:40") and an interval ("10-20min"), on two rows of one column.
var (
	freqWindowCell   = regexp.MustCompile(`^(\d{1,2}:\d{2})-(\d{1,2}:\d{2})$`)
	freqIntervalCell = regexp.MustCompile(`^(\d{1,2})(?:\s*-\s*(\d{1,2}))?\s*min$`)
)

func isTimeCell(s string) bool {
	return s == "" || timeCell.MatchString(s)
}

// Folds a window or interval cell into freq and reports whether s was one; headway-only directions (M26) have only these.
func applyFreqCell(freq **models.Frequency, s string) bool {
	if m := freqWindowCell.FindStringSubmatch(s); m != nil {
		if *freq == nil {
			*freq = &models.Frequency{}
		}
		(*freq).Start, (*freq).End = m[1], m[2]
		return true
	}
	if m := freqIntervalCell.FindStringSubmatch(s); m != nil {
		if *freq == nil {
			*freq = &models.Frequency{}
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

func parseCSV(data []byte) (*csvDay, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	scanner := bufio.NewScanner(bytes.NewReader(data))

	t := &csvDay{DaySchedule: models.DaySchedule{Entries: []models.TimetableEntry{}}}

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
			t.routeLongName = right
			continue
		case "service_name":
			t.ServiceName = right
			continue
		case "service_start":
			t.ServiceStart = right
			continue
		case "in_stop_name":
			t.inStopName = right
			continue
		case "out_stop_name":
			t.outStopName = right
			continue
		}

		// Blank the headway cell so the other column's time on that row is kept.
		if applyFreqCell(&t.InFrequency, left) {
			left = ""
		}
		if applyFreqCell(&t.OutFrequency, right) {
			right = ""
		}

		if !isTimeCell(left) || !isTimeCell(right) {
			continue
		}
		t.Entries = append(t.Entries, models.TimetableEntry{
			DepartureIn:  left,
			DepartureOut: right,
		})
	}

	feeds.NormalizeEntries(t.Entries)
	return t, scanner.Err()
}
