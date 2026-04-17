package ctp_cj

import (
	"bufio"
	"bytes"
	"fmt"
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

func ParseTimetableCSV(data []byte) (*ParsedTimetable, error) {
	// Some CTP CSVs (e.g. 27, M27) are served with a UTF-8 BOM, which turns
	// the first key into "\ufeffroute_long_name" and trips the meta check.
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	scanner := bufio.NewScanner(bytes.NewReader(data))

	meta := func(key string) (string, error) {
		if !scanner.Scan() {
			return "", fmt.Errorf("unexpected EOF, expected %q row", key)
		}
		parts := strings.SplitN(scanner.Text(), ",", 2)
		if len(parts) != 2 || parts[0] != key {
			return "", fmt.Errorf("expected key %q, got line %q", key, scanner.Text())
		}
		return parts[1], nil
	}

	t := &ParsedTimetable{}
	var err error

	if t.RouteLongName, err = meta("route_long_name"); err != nil {
		return nil, err
	}
	if t.ServiceName, err = meta("service_name"); err != nil {
		return nil, err
	}
	if t.ServiceStart, err = meta("service_start"); err != nil {
		return nil, err
	}
	if t.InStopName, err = meta("in_stop_name"); err != nil {
		return nil, err
	}
	if t.OutStopName, err = meta("out_stop_name"); err != nil {
		return nil, err
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed time row: %q", line)
		}
		t.Entries = append(t.Entries, TimetableEntry{
			DepartureIn:  parts[0],
			DepartureOut: parts[1],
		})
	}

	return t, scanner.Err()
}
