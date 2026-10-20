package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/models"
	"database/sql"
	"encoding/json"
	"fmt"
)

// StoreTimetable replaces a line's timetable and keeps its version in the archive.
func (d *Data) StoreTimetable(t *models.Timetable) error {
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

	_, err = d.db.Exec(`
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

	d.archiveTimetable(t, string(weekdaysJSON), string(saturdayJSON), string(sundayJSON))
	return nil
}

// Rows are keyed by stop_sequence, so a trip that lost stops would keep its old tail without the delete.
func (d *Data) StoreStopTimes(stopTimes []models.StopTime) error {
	clear := func(tx *sql.Tx) error {
		for _, id := range TripIDs(stopTimes) {
			if _, err := tx.Exec(`DELETE FROM stop_times WHERE trip_id = ?`, id); err != nil {
				return fmt.Errorf("error clearing stop times: %w", err)
			}
		}
		return nil
	}
	err := database.BatchReplace(d.db, clear, `
		INSERT OR REPLACE INTO stop_times
		(trip_id, stop_id, offset_arrival_time, stop_sequence, stop_headsign, route_short_name, stop_lat, stop_lon)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		func(stmt *sql.Stmt) error {
			for _, st := range stopTimes {
				if _, err := stmt.Exec(st.TripID, st.StopID, st.OffsetArrivalTime, st.StopSequence, st.StopHeadsign, st.RouteShortName, st.StopLat, st.StopLon); err != nil {
					return fmt.Errorf("error inserting stop time: %w", err)
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	d.archiveStopTimes(stopTimes)
	return nil
}

// ScrubStopTimes drops the stop times of patterns the network no longer has.
func (d *Data) ScrubStopTimes() (int64, error) {
	res, err := d.db.Exec(`DELETE FROM stop_times WHERE trip_id NOT IN (SELECT trip_id FROM trips)`)
	if err != nil {
		return 0, fmt.Errorf("scrub orphans: %w", err)
	}
	return res.RowsAffected()
}

// TripIDs is the patterns stopTimes belong to, in the order they first appear.
func TripIDs(stopTimes []models.StopTime) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, st := range stopTimes {
		if !seen[st.TripID] {
			seen[st.TripID] = true
			ids = append(ids, st.TripID)
		}
	}
	return ids
}
