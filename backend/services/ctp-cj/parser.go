package ctp_cj

import (
	"bufio"
	"bytes"
	"regexp"
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

// timeCell matches a single HH:MM cell, optionally followed by annotations
// like "*" that CTP uses to flag conditional trips. Either side of the pair
// may be empty (one-way trips), which we preserve as "".
var timeCell = regexp.MustCompile(`^\s*\d{1,2}:\d{2}\S*\s*$`)

func isTimeCell(s string) bool {
	return s == "" || timeCell.MatchString(s)
}

func ParseTimetableCSV(data []byte) (*ParsedTimetable, error) {
	// Some CTP CSVs (e.g. 27, M27) are served with a UTF-8 BOM, which turns
	// the first key into "\ufeffroute_long_name" and trips the meta check.
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
			// Single-column rows like a bare "Nu circula" — treat the day as empty.
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

		// Anything else is expected to be a departure pair. Either cell may be
		// empty (one-way trips) and CTP sometimes annotates times (e.g. "07:20*").
		// Rows like "Nu circula,Nu circula" signal "no service that day" — skip.
		if !isTimeCell(left) || !isTimeCell(right) {
			continue
		}
		t.Entries = append(t.Entries, TimetableEntry{
			DepartureIn:  left,
			DepartureOut: right,
		})
	}

	return t, scanner.Err()
}
