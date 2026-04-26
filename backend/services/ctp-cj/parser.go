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

type ParsedTimetable struct {
	RouteLongName string
	ServiceName   string
	ServiceStart  string
	InStopName    string
	OutStopName   string
	Entries       []TimetableEntry
}

// Accept HH:MM and annotated values like 07:20*.
var timeCell = regexp.MustCompile(`^\s*\d{1,2}:\d{2}\S*\s*$`)

func isTimeCell(s string) bool {
	return s == "" || timeCell.MatchString(s)
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
