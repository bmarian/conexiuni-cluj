package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/sources/ctpcsv"
	"encoding/json"
	"fmt"
)

func (city *City) GetTimetable(routeShortName string) (*models.Timetable, error) {
	return city.getTimetableFromDB(routeShortName)
}

func (city *City) getTimetableFromDB(routeShortName string) (*models.Timetable, error) {
	var t models.Timetable
	var weekdaysJSON, saturdayJSON, sundayJSON string

	err := city.DB.QueryRow(
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

// Cached rows may predate 24+ hour notation.
func normalizeDaySchedule(d *models.DaySchedule) {
	prevIn, prevOut := -1, -1
	offIn, offOut := 0, 0
	for i := range d.Entries {
		d.Entries[i].DepartureIn = ctpcsv.NormalizeTime(d.Entries[i].DepartureIn, &prevIn, &offIn)
		d.Entries[i].DepartureOut = ctpcsv.NormalizeTime(d.Entries[i].DepartureOut, &prevOut, &offOut)
	}
}

func (city *City) storeTimetableInDB(t *models.Timetable) error {
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

	_, err = city.DB.Exec(`
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

	city.archiveTimetable(t, string(weekdaysJSON), string(saturdayJSON), string(sundayJSON))
	return nil
}
