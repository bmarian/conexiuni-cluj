package handlers

import (
	"archive/zip"
	"bytes"
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

var (
	gtfsMu  sync.RWMutex
	gtfsZip []byte
)

// BuildGTFSZip generates a GTFS zip archive from the currently cached data
// and stores it in memory for serving.
func BuildGTFSZip(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) error {
	start := time.Now()
	log.Println("gtfs: building GTFS zip from cached data")

	routes, err := GetRoutes(tranzyClient, cacheTimes.RouteCacheShelfLife, RouteFilter{})
	if err != nil {
		return fmt.Errorf("gtfs: failed to get routes: %w", err)
	}

	stops, err := GetStops(tranzyClient, cacheTimes.StopCacheShelfLife, StopFilter{})
	if err != nil {
		return fmt.Errorf("gtfs: failed to get stops: %w", err)
	}

	trips, err := GetTrips(tranzyClient, cacheTimes.TripCacheShelfLife, TripFilter{})
	if err != nil {
		return fmt.Errorf("gtfs: failed to get trips: %w", err)
	}

	shapes, err := GetShapes(tranzyClient, cacheTimes.ShapeCacheShelfLife, ShapeFilter{})
	if err != nil {
		return fmt.Errorf("gtfs: failed to get shapes: %w", err)
	}

	apiStopTimes, err := getAPIStopTimes(tranzyClient, cacheTimes.APIStopTimeCacheShelfLife, APIStopTimeFilter{})
	if err != nil {
		return fmt.Errorf("gtfs: failed to get api_stop_times: %w", err)
	}

	// Build route lookup: route_id → Route
	routeByID := make(map[int]models.Route, len(routes))
	for _, r := range routes {
		routeByID[r.RouteID] = r
	}

	// Collect timetables for all routes
	timetables := make(map[string]*models.Timetable, len(routes))
	for _, r := range routes {
		tt, err := GetTimetable(ctpCjClient, cacheTimes.TimetableCacheShelfLife, r.RouteShortName)
		if err == nil && tt != nil {
			timetables[r.RouteShortName] = tt
		}
	}

	// Group API stop times by trip_id, sorted by stop_sequence
	type stopTimeEntry struct {
		StopID       int
		StopSequence int
	}
	stopTimesByTrip := make(map[string][]stopTimeEntry, len(trips))
	for _, ast := range apiStopTimes {
		tid := NormalizeTripID(ast.TripID)
		stopTimesByTrip[tid] = append(stopTimesByTrip[tid], stopTimeEntry{
			StopID:       ast.StopID,
			StopSequence: ast.StopSequence,
		})
	}

	// Build trip lookup
	tripByID := make(map[string]models.Trip, len(trips))
	for _, t := range trips {
		tripByID[NormalizeTripID(t.TripID)] = t
	}

	// Build stop lookup
	stopByID := make(map[int]models.Stop, len(stops))
	for _, s := range stops {
		stopByID[s.StopID] = s
	}

	// Build shape lookup
	shapesByID := make(map[string][]models.Shape)
	for _, s := range shapes {
		shapesByID[s.ShapeID] = append(shapesByID[s.ShapeID], s)
	}

	// Compute offset for each stop in each trip
	type enrichedStopTime struct {
		TripID       string
		StopID       int
		StopSequence int
		OffsetSec    float64 // seconds from first stop departure
	}
	var enrichedSTs []enrichedStopTime
	for tid, sts := range stopTimesByTrip {
		t, ok := tripByID[tid]
		if !ok {
			continue
		}
		tripShapes := shapesByID[t.ShapeID]
		var prevStop *models.Stop
		cumOffset := 0.0
		for _, st := range sts {
			curStop := stopByID[st.StopID]
			if prevStop != nil && len(tripShapes) > 0 {
				segOffset := calculateStopOffset(*prevStop, curStop, tripShapes)
				cumOffset += segOffset
			}
			enrichedSTs = append(enrichedSTs, enrichedStopTime{
				TripID:       tid,
				StopID:       st.StopID,
				StopSequence: st.StopSequence,
				OffsetSec:    cumOffset,
			})
			prevStop = &curStop
		}
	}

	// Group enriched stop times by trip
	enrichedByTrip := make(map[string][]enrichedStopTime)
	for _, est := range enrichedSTs {
		enrichedByTrip[est.TripID] = append(enrichedByTrip[est.TripID], est)
	}

	// Determine calendar date range: today + 30 days
	now := time.Now()
	calStart := now.Format("20060102")
	calEnd := now.AddDate(0, 0, 30).Format("20060102")

	// Service IDs
	const (
		serviceWeekday = "weekday"
		serviceSat     = "saturday"
		serviceSun     = "sunday"
	)

	// Build the zip
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// agency.txt
	if err := writeCSV(zw, "agency.txt", [][]string{
		{"agency_id", "agency_name", "agency_url", "agency_timezone", "agency_lang"},
		{"1", "CTP Cluj-Napoca", "https://ctpcj.ro", "Europe/Bucharest", "ro"},
	}); err != nil {
		return fmt.Errorf("gtfs: agency.txt: %w", err)
	}

	// calendar.txt
	if err := writeCSV(zw, "calendar.txt", [][]string{
		{"service_id", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "start_date", "end_date"},
		{serviceWeekday, "1", "1", "1", "1", "1", "0", "0", calStart, calEnd},
		{serviceSat, "0", "0", "0", "0", "0", "1", "0", calStart, calEnd},
		{serviceSun, "0", "0", "0", "0", "0", "0", "1", calStart, calEnd},
	}); err != nil {
		return fmt.Errorf("gtfs: calendar.txt: %w", err)
	}

	// routes.txt
	routeRows := [][]string{
		{"route_id", "agency_id", "route_short_name", "route_long_name", "route_type", "route_color"},
	}
	for _, r := range routes {
		color := strings.TrimPrefix(r.RouteColor, "#")
		routeRows = append(routeRows, []string{
			strconv.Itoa(r.RouteID),
			"1",
			r.RouteShortName,
			r.RouteLongName,
			strconv.Itoa(int(r.RouteType)),
			color,
		})
	}
	if err := writeCSV(zw, "routes.txt", routeRows); err != nil {
		return fmt.Errorf("gtfs: routes.txt: %w", err)
	}

	// stops.txt
	stopRows := [][]string{
		{"stop_id", "stop_name", "stop_lat", "stop_lon", "location_type"},
	}
	for _, s := range stops {
		stopRows = append(stopRows, []string{
			strconv.Itoa(s.StopID),
			s.StopName,
			strconv.FormatFloat(s.StopLat, 'f', 6, 64),
			strconv.FormatFloat(s.StopLon, 'f', 6, 64),
			strconv.Itoa(int(s.LocationType)),
		})
	}
	if err := writeCSV(zw, "stops.txt", stopRows); err != nil {
		return fmt.Errorf("gtfs: stops.txt: %w", err)
	}

	// trips.txt — expand each trip into up to 3 service variants, each with
	// N departures from the timetable. Each departure becomes a unique GTFS trip.
	tripRows := [][]string{
		{"route_id", "service_id", "trip_id", "trip_headsign", "direction_id", "shape_id"},
	}

	// stop_times.txt
	stRows := [][]string{
		{"trip_id", "arrival_time", "departure_time", "stop_id", "stop_sequence"},
	}

	type serviceInfo struct {
		serviceID  string
		departures []string // HH:MM times for direction 0
	}

	for _, trip := range trips {
		tid := NormalizeTripID(trip.TripID)
		route, ok := routeByID[trip.RouteID]
		if !ok {
			continue
		}

		enrichedStops := enrichedByTrip[tid]
		if len(enrichedStops) == 0 {
			continue
		}

		tt := timetables[route.RouteShortName]
		if tt == nil {
			continue
		}

		// Determine which departure column to use based on direction
		services := []serviceInfo{
			{serviceID: serviceWeekday, departures: extractDepartures(tt.Weekdays, trip.DirectionID)},
			{serviceID: serviceSat, departures: extractDepartures(tt.Saturday, trip.DirectionID)},
			{serviceID: serviceSun, departures: extractDepartures(tt.Sunday, trip.DirectionID)},
		}

		for _, svc := range services {
			if len(svc.departures) == 0 {
				continue
			}

			for depIdx, depTime := range svc.departures {
				baseSec, ok := parseTimeToSeconds(depTime)
				if !ok {
					continue
				}

				gtfsTripID := fmt.Sprintf("%s_%s_%d", tid, svc.serviceID, depIdx)

				tripRows = append(tripRows, []string{
					strconv.Itoa(trip.RouteID),
					svc.serviceID,
					gtfsTripID,
					trip.TripHeadsign,
					strconv.Itoa(int(trip.DirectionID)),
					trip.ShapeID,
				})

				for _, est := range enrichedStops {
					arrSec := baseSec + int(est.OffsetSec)
					timeStr := secondsToGTFSTime(arrSec)
					stRows = append(stRows, []string{
						gtfsTripID,
						timeStr,
						timeStr,
						strconv.Itoa(est.StopID),
						strconv.Itoa(est.StopSequence),
					})
				}
			}
		}
	}

	if err := writeCSV(zw, "trips.txt", tripRows); err != nil {
		return fmt.Errorf("gtfs: trips.txt: %w", err)
	}

	if err := writeCSV(zw, "stop_times.txt", stRows); err != nil {
		return fmt.Errorf("gtfs: stop_times.txt: %w", err)
	}

	// shapes.txt
	shapeRows := [][]string{
		{"shape_id", "shape_pt_lat", "shape_pt_lon", "shape_pt_sequence", "shape_dist_traveled"},
	}
	for _, s := range shapes {
		distStr := ""
		if s.ShapeDistTraveled >= 0 {
			distStr = strconv.FormatFloat(s.ShapeDistTraveled, 'f', 4, 64)
		}
		shapeRows = append(shapeRows, []string{
			s.ShapeID,
			strconv.FormatFloat(s.ShapePtLat, 'f', 6, 64),
			strconv.FormatFloat(s.ShapePtLon, 'f', 6, 64),
			strconv.Itoa(s.ShapePtSequence),
			distStr,
		})
	}
	if err := writeCSV(zw, "shapes.txt", shapeRows); err != nil {
		return fmt.Errorf("gtfs: shapes.txt: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("gtfs: failed to close zip: %w", err)
	}

	gtfsMu.Lock()
	gtfsZip = buf.Bytes()
	gtfsMu.Unlock()

	log.Printf("gtfs: zip built (%d bytes, %d routes, %d stops, %d trips, %d stop_time rows, %d shape points) in %s",
		len(gtfsZip), len(routes), len(stops), len(tripRows)-1, len(stRows)-1, len(shapeRows)-1,
		time.Since(start).Round(time.Millisecond))
	return nil
}

// HandleGTFSExport serves the pre-built GTFS zip.
func HandleGTFSExport(c fiber.Ctx) error {
	gtfsMu.RLock()
	data := gtfsZip
	gtfsMu.RUnlock()

	if data == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "GTFS feed not yet available"})
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename=gtfs.zip")
	c.Set("Cache-Control", staticCacheControl)
	return c.Send(data)
}

// extractDepartures returns departure times for the given direction from a DaySchedule.
// direction 0 → DepartureOut, direction 1 → DepartureIn.
func extractDepartures(day models.DaySchedule, direction models.DirectionType) []string {
	var out []string
	for _, e := range day.Entries {
		var dep string
		if direction == models.Outbound {
			dep = e.DepartureOut
		} else {
			dep = e.DepartureIn
		}
		dep = strings.TrimSpace(dep)
		if dep != "" {
			out = append(out, dep)
		}
	}
	return out
}

// parseTimeToSeconds parses "HH:MM" (possibly >23) to total seconds since midnight.
func parseTimeToSeconds(t string) (int, bool) {
	// Strip any trailing non-digit suffixes (e.g. "05:30A")
	t = strings.TrimSpace(t)
	parts := strings.SplitN(t, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	// Extract only digits from minute part
	minStr := parts[1]
	for i, c := range minStr {
		if c < '0' || c > '9' {
			minStr = minStr[:i]
			break
		}
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(minStr)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return h*3600 + m*60, true
}

// secondsToGTFSTime formats total seconds as HH:MM:SS (GTFS allows >23 hours).
func secondsToGTFSTime(sec int) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func writeCSV(zw *zip.Writer, name string, rows [][]string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	return cw.WriteAll(rows)
}
