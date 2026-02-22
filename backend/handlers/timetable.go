package handlers

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	ctp_cj "conexiuni-cluj/services/ctp-cj"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
)

func GetTimetable(c fiber.Ctx, ctpCjClient *ctp_cj.Client, shelfLife time.Duration, routeShortName string) error {
	cacheID := "TIMETABLE_" + routeShortName
	return HandleCached(c, cacheID, shelfLife,
		func() (*models.Timetable, error) { return getTimetableFromDB(routeShortName) },
		func() (*models.Timetable, error) { return fetchTimetable(ctpCjClient, routeShortName) },
		storeTimetableInDB,
		CacheOpts[*models.Timetable]{},
	)
}

func fetchTimetable(ctpCjClient *ctp_cj.Client, routeShortName string) (*models.Timetable, error) {
	weekdays, saturday, sunday, err := ctpCjClient.FetchTimetable(routeShortName)
	if err != nil {
		return nil, err
	}

	t := &models.Timetable{
		RouteShortName: routeShortName,
		Weekdays:       []models.TimetableEntry{},
		Saturday:       []models.TimetableEntry{},
		Sunday:         []models.TimetableEntry{},
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
		t.Weekdays = toModelEntries(weekdays.Entries)
	}
	if saturday != nil {
		t.Saturday = toModelEntries(saturday.Entries)
	}
	if sunday != nil {
		t.Sunday = toModelEntries(sunday.Entries)
	}

	return t, nil
}

func toModelEntries(entries []ctp_cj.TimetableEntry) []models.TimetableEntry {
	out := make([]models.TimetableEntry, len(entries))
	for i, e := range entries {
		out[i] = models.TimetableEntry{
			DepartureIn:  e.DepartureIn,
			DepartureOut: e.DepartureOut,
		}
	}
	return out
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

	return &t, nil
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
