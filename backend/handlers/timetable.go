package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctp_cj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

func GetTimetable(ctpCjClient *ctp_cj.Client, tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, routeShortName string) (*models.Timetable, error) {
	cacheID := "TIMETABLE_" + routeShortName
	return HandleCached(cacheID, cacheTimes.TimetableCacheShelfLife,
		func() (*models.Timetable, error) { return getTimetableFromDB(routeShortName) },
		func() (*models.Timetable, error) {
			return fetchTimetable(ctpCjClient, tranzyClient, cacheTimes, routeShortName)
		},
		storeTimetableInDB,
		CacheOpts[*models.Timetable]{Optimize: true},
	)
}

func fetchTimetable(ctpCjClient *ctp_cj.Client, tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, routeShortName string) (*models.Timetable, error) {
	weekdays, saturday, sunday, err := ctpCjClient.FetchTimetable(routeShortName)
	if err != nil {
		return nil, err
	}

	t := &models.Timetable{
		RouteShortName: routeShortName,
		Weekdays:       models.DaySchedule{Entries: []models.TimetableEntry{}},
		Saturday:       models.DaySchedule{Entries: []models.TimetableEntry{}},
		Sunday:         models.DaySchedule{Entries: []models.TimetableEntry{}},
	}

	for _, parsed := range []*ctp_cj.ParsedTimetable{weekdays, saturday, sunday} {
		if parsed != nil {
			t.RouteLongName = parsed.RouteLongName
			t.InStopName = parsed.InStopName
			t.OutStopName = parsed.OutStopName
			break
		}
	}

	if weekdays != nil {
		t.Weekdays = toDaySchedule(weekdays)
	}
	if saturday != nil {
		t.Saturday = toDaySchedule(saturday)
	}
	if sunday != nil {
		t.Sunday = toDaySchedule(sunday)
	}

	alignTimetableToDirectionIDs(tranzyClient, cacheTimes, t)

	return t, nil
}

// alignTimetableToDirectionIDs ensures DepartureIn / InStopName line up with
// Tranzy's direction_id=0. CTP-CJ's "in"/"out" labels are independent of
// Tranzy's direction IDs, so for some routes (e.g. line 57) the frontend's
// assumption "departure_in ↔ direction 0" renders the wrong column. We compare
// CTP's stop names against the first stop of trips ending _0 vs _1 and swap
// columns when the swapped arrangement is the better match. Falls back to as-is
// when matches are absent or tied.
func alignTimetableToDirectionIDs(tranzyClient *tranzy.Client, cacheTimes models.CacheTimes, t *models.Timetable) {
	if t == nil || (t.InStopName == "" && t.OutStopName == "") {
		return
	}
	stopTimes, err := GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &t.RouteShortName})
	if err != nil {
		log.Printf("timetable %s: skip direction alignment (stop_times error: %v)", t.RouteShortName, err)
		return
	}
	firstStop0 := firstStopOfDirection(stopTimes, "_0")
	firstStop1 := firstStopOfDirection(stopTimes, "_1")
	if firstStop0 == "" || firstStop1 == "" {
		return
	}

	keep := stopMatchScore(t.InStopName, firstStop0) + stopMatchScore(t.OutStopName, firstStop1)
	swap := stopMatchScore(t.InStopName, firstStop1) + stopMatchScore(t.OutStopName, firstStop0)

	if swap > keep {
		log.Printf("timetable %s: swapping directions (CTP in=%q out=%q ↔ Tranzy dir0=%q dir1=%q)",
			t.RouteShortName, t.InStopName, t.OutStopName, firstStop0, firstStop1)
		t.InStopName, t.OutStopName = t.OutStopName, t.InStopName
		swapDayScheduleColumns(&t.Weekdays)
		swapDayScheduleColumns(&t.Saturday)
		swapDayScheduleColumns(&t.Sunday)
	}
}

func firstStopOfDirection(stopTimes []models.StopTime, tripSuffix string) string {
	var best *models.StopTime
	for i := range stopTimes {
		st := &stopTimes[i]
		if !strings.HasSuffix(st.TripID, tripSuffix) {
			continue
		}
		if best == nil || st.StopSequence < best.StopSequence {
			best = st
		}
	}
	if best == nil {
		return ""
	}
	return best.StopHeadsign
}

func swapDayScheduleColumns(d *models.DaySchedule) {
	for i := range d.Entries {
		d.Entries[i].DepartureIn, d.Entries[i].DepartureOut = d.Entries[i].DepartureOut, d.Entries[i].DepartureIn
	}
}

func toDaySchedule(p *ctp_cj.ParsedTimetable) models.DaySchedule {
	entries := make([]models.TimetableEntry, len(p.Entries))
	for i, e := range p.Entries {
		entries[i] = models.TimetableEntry{
			DepartureIn:  e.DepartureIn,
			DepartureOut: e.DepartureOut,
		}
	}
	return models.DaySchedule{
		ServiceName:  p.ServiceName,
		ServiceStart: p.ServiceStart,
		Entries:      entries,
	}
}

func getTimetableFromDB(routeShortName string) (*models.Timetable, error) {
	var t models.Timetable
	var weekdaysJSON, saturdayJSON, sundayJSON string

	err := database.DB.QueryRow(
		`SELECT route_short_name, route_long_name, in_stop_name, out_stop_name, weekdays, saturday, sunday
		 FROM timetable WHERE route_short_name = ?`,
		routeShortName,
	).Scan(
		&t.RouteShortName,
		&t.RouteLongName,
		&t.InStopName,
		&t.OutStopName,
		&weekdaysJSON,
		&saturdayJSON,
		&sundayJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying timetable: %w", err)
	}

	if err := json.Unmarshal([]byte(weekdaysJSON), &t.Weekdays); err != nil {
		return nil, fmt.Errorf("error unmarshalling weekdays: %w", err)
	}
	if err := json.Unmarshal([]byte(saturdayJSON), &t.Saturday); err != nil {
		return nil, fmt.Errorf("error unmarshalling saturday: %w", err)
	}
	if err := json.Unmarshal([]byte(sundayJSON), &t.Sunday); err != nil {
		return nil, fmt.Errorf("error unmarshalling sunday: %w", err)
	}

	normalizeDaySchedule(&t.Weekdays)
	normalizeDaySchedule(&t.Saturday)
	normalizeDaySchedule(&t.Sunday)

	return &t, nil
}

// normalizeDaySchedule applies GTFS 24+ hour notation to entries read from the
// DB cache, which may pre-date the parser-level fix.
func normalizeDaySchedule(d *models.DaySchedule) {
	prevIn, prevOut := -1, -1
	offIn, offOut := 0, 0
	for i := range d.Entries {
		d.Entries[i].DepartureIn = normalizeTimetableTime(d.Entries[i].DepartureIn, &prevIn, &offIn)
		d.Entries[i].DepartureOut = normalizeTimetableTime(d.Entries[i].DepartureOut, &prevOut, &offOut)
	}
}

func normalizeTimetableTime(s string, prev *int, offset *int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
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

func storeTimetableInDB(t *models.Timetable) error {
	weekdaysJSON, err := json.Marshal(t.Weekdays)
	if err != nil {
		return fmt.Errorf("error marshalling weekdays: %w", err)
	}
	saturdayJSON, err := json.Marshal(t.Saturday)
	if err != nil {
		return fmt.Errorf("error marshalling saturday: %w", err)
	}
	sundayJSON, err := json.Marshal(t.Sunday)
	if err != nil {
		return fmt.Errorf("error marshalling sunday: %w", err)
	}

	_, err = database.DB.Exec(`
		INSERT OR REPLACE INTO timetable
		(route_short_name, route_long_name, in_stop_name, out_stop_name, weekdays, saturday, sunday)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.RouteShortName,
		t.RouteLongName,
		t.InStopName,
		t.OutStopName,
		string(weekdaysJSON),
		string(saturdayJSON),
		string(sundayJSON),
	)
	if err != nil {
		return fmt.Errorf("error storing timetable: %w", err)
	}

	return nil
}
