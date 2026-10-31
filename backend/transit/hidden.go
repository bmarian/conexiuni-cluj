package transit

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/feeds"
	"conexiuni-cluj/models"
	"database/sql"
	"maps"
	"slices"
	"time"
)

func (d *Data) UseHiddenStops(seed []int) error {
	if err := d.store.SeedHiddenStops(seed, time.Now()); err != nil {
		return err
	}
	return d.readHiddenStops()
}

func (d *Data) readHiddenStops() error {
	ids, err := d.store.ReadHiddenStops()
	if err != nil {
		return err
	}
	hidden := make(map[int]bool, len(ids))
	for _, id := range ids {
		hidden[id] = true
	}
	d.hidden.Store(&hidden)
	return nil
}

func (d *Data) SetStopsHidden(ids []int, hidden bool) error {
	if err := d.store.SaveHiddenStops(ids, hidden, time.Now()); err != nil {
		return err
	}
	return d.readHiddenStops()
}

func (d *Data) hiddenStops() map[int]bool {
	if hidden := d.hidden.Load(); hidden != nil {
		return *hidden
	}
	return nil
}

func (d *Data) HiddenStops() []int { return slices.Sorted(maps.Keys(d.hiddenStops())) }

func (d *Data) StopHidden(id int) bool { return d.hiddenStops()[id] }

// Rows in trip and stop order; the next stop takes over a hidden stop's time.
func (d *Data) Passenger(rows []models.StopTime) []models.StopTime {
	hidden := d.hiddenStops()
	if len(hidden) == 0 {
		return rows
	}
	out := make([]models.StopTime, 0, len(rows))
	trip, carried := "", 0.0
	for _, st := range rows {
		if st.TripID != trip {
			trip, carried = st.TripID, 0
		}
		if hidden[st.StopID] {
			carried += st.OffsetArrivalTime
			continue
		}
		st.OffsetArrivalTime += carried
		carried = 0
		out = append(out, st)
	}
	return out
}

// timetableStarts holds the trips whose timetable names their first visible stop, by that stop's position.
func (d *Data) timetableStarts() (map[string]int, error) {
	out := map[string]int{}
	hidden := d.hiddenStops()
	if len(hidden) == 0 {
		return out, nil
	}
	stops, err := d.Stops(StopFilter{})
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(stops))
	for _, s := range stops {
		names[s.StopID] = s.StopName
	}
	calls, err := d.TripStops()
	if err != nil {
		return nil, err
	}
	byTrip := map[string][]int{}
	for _, c := range calls {
		byTrip[c.TripID] = append(byTrip[c.TripID], c.StopID)
	}
	err = database.QueryEach(d.db, `
		SELECT t.trip_id, t.direction_id, tt.in_stop_name, tt.out_stop_name
		FROM trips t
		JOIN routes r ON r.route_id = t.route_id
		JOIN timetable tt ON tt.route_short_name = r.route_short_name`, nil, func(rows *sql.Rows) error {
		var trip, in, departing string
		var direction int
		if err := rows.Scan(&trip, &direction, &in, &departing); err != nil {
			return err
		}
		if direction == 0 {
			departing = in
		}
		trail := byTrip[trip]
		first := 0
		for first < len(trail) && hidden[trail[first]] {
			first++
		}
		if first > 0 && first < len(trail) && feeds.NormalizeStopLabel(departing) == feeds.NormalizeStopLabel(names[trail[first]]) {
			out[trip] = first
		}
		return nil
	})
	return out, err
}

func (d *Data) RoutesAtStops(ids []int) (map[int][]int, error) {
	out := make(map[int][]int, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := placeholders(ids)
	err := database.QueryEach(d.db, `
		SELECT DISTINCT ts.stop_id, t.route_id
		FROM trip_stops ts
		JOIN trips t ON t.trip_id = ts.trip_id
		WHERE ts.stop_id IN (`+in+`)
		ORDER BY t.route_id`, args, func(rows *sql.Rows) error {
		var stop, route int
		if err := rows.Scan(&stop, &route); err != nil {
			return err
		}
		out[stop] = append(out[stop], route)
		return nil
	})
	return out, err
}
