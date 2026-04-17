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

	return t, scanner.Err()
}
