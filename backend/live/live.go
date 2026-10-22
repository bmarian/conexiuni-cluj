package live

import (
	"conexiuni-cluj/calendar"
	"conexiuni-cluj/database"
	"conexiuni-cluj/jobs"
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"conexiuni-cluj/transit"
	"database/sql"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

// One row per (trip, stop), rebuilt on every vehicle poll.

const (
	liveScheduleHorizon = 8 * time.Hour
	liveEntriesPerRow   = 3
	liveTripsRefresh    = 5 * time.Minute
)

// At is unix seconds.
type liveEntry struct {
	At        int64 `json:"at"`
	Live      bool  `json:"live"`
	Soon      bool  `json:"soon,omitempty"`
	VehicleID int   `json:"vehicle_id,omitempty"`
	kind      predictions.EtaKind
}

type liveRow struct {
	TripID  string      `json:"trip_id"`
	StopID  int         `json:"stop_id"`
	Entries []liveEntry `json:"entries"`
}

type liveVehicle struct {
	ID          int     `json:"id"`
	TripID      string  `json:"trip_id"`
	RouteID     int     `json:"route_id"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Heading     float64 `json:"heading"`
	Speed       float64 `json:"speed"`
	Timestamp   string  `json:"timestamp"`
	Label       string  `json:"label,omitempty"`
	VehicleType int     `json:"vehicle_type,omitempty"`
	Bike        string  `json:"bike_accessible,omitempty"`
	Wheelchair  string  `json:"wheelchair_accessible,omitempty"`
}

type liveTable struct {
	AsOf           time.Time
	rowsByTrip     map[string][]liveRow
	rowsByStop     map[int][]liveRow
	vehiclesByTrip map[string][]liveVehicle
}

type liveTrip struct {
	trip          *predictions.Trip
	routeID       int
	cumulativeSec []float64
	departures    map[string][]int
	// What the timetable plans between arriving and leaving the departure terminus, per kind of day.
	layoverMin map[string]float64
}

var (
	liveClock = time.Now
	// How often the vehicles are polled; the replay harness overrides it.
	livePollInterval = func(eng *Engine) time.Duration {
		if eng.hub == nil {
			return 0
		}
		return eng.hub.CurrentInterval()
	}
)

// Learned from schedule delays: how far single runs stray from their corrected timetable.
type liveRules struct {
	graceSec   float64
	slotWindow time.Duration
}

func (eng *Engine) learnLiveRules() liveRules {
	lateness, err := database.QueryRows(eng.DB, `SELECT spread_sec FROM schedule_delay_profiles WHERE spread_sec > 0 ORDER BY 1`, nil,
		func(rows *sql.Rows) (float64, error) {
			var d float64
			err := rows.Scan(&d)
			return d, err
		})
	if err != nil {
		eng.log("live: schedule delays: %v", err)
	}
	// A run that still reads "now" on a screen (half a minute late) is always listed.
	rules := liveRules{graceSec: 30, slotWindow: time.Minute}
	if len(lateness) > 0 {
		rules.graceSec = math.Max(30, nearestRank(lateness, 0.75))
		rules.slotWindow = time.Duration(math.Max(60, nearestRank(lateness, 0.9)) * float64(time.Second))
	}
	return rules
}

func (eng *Engine) StartLive(r *jobs.Runner) {
	r.Go(func() {
		for {
			eng.refreshLiveTrips(false)
			if !r.Sleep(liveTripsRefresh) {
				return
			}
		}
	})
}

// Callers wait for a rebuild in progress, so the first table never sees empty trips.
func (eng *Engine) refreshLiveTrips(force bool) {
	eng.liveRefreshMu.Lock()
	defer eng.liveRefreshMu.Unlock()
	now := liveClock().In(eng.Location)
	period := profilePeriod(now)
	day := now.Format("2006-01-02")
	eng.liveTrips.RLock()
	fresh := eng.liveTrips.byTrip != nil && eng.liveTrips.period == period && eng.liveTrips.day == day
	eng.liveTrips.RUnlock()
	if fresh && !force {
		return
	}
	routes, err := eng.Routes(transit.RouteFilter{})
	if err != nil {
		eng.log("live: routes: %v", err)
		return
	}
	started := time.Now()
	byTrip := make(map[string]*liveTrip)
	routeIDs := make(map[string]int, len(routes))
	for _, route := range routes {
		routeIDs[route.RouteShortName] = route.RouteID
		for _, lt := range eng.buildLiveTrips(route, now) {
			byTrip[lt.trip.ID] = lt
		}
	}
	rules := eng.learnLiveRules()
	eng.liveTrips.Lock()
	eng.liveTrips.byTrip, eng.liveTrips.period, eng.liveTrips.day, eng.liveTrips.rules = byTrip, period, day, rules
	eng.liveTrips.routeIDs = routeIDs
	eng.liveTrips.Unlock()
	eng.log("live: %d trips prepared in %s, grace %.0fs, reminder window %s", len(byTrip), time.Since(started).Round(time.Millisecond), rules.graceSec, rules.slotWindow)
}

func (eng *Engine) buildLiveTrips(route models.Route, now time.Time) []*liveTrip {
	rsn := route.RouteShortName
	stopTimes, err := eng.GetStopTimesAt(transit.StopTimeFilter{RouteShortName: &rsn}, now)
	if err != nil || len(stopTimes) == 0 {
		return nil
	}
	timetable, _ := eng.Timetable(rsn)
	turnarounds := eng.routeTurnarounds(route.RouteID, stopTimes)
	var out []*liveTrip
	for dir, suffix := range []string{models.OutgoingSuffix, models.IncomingSuffix} {
		tripID := strconv.Itoa(route.RouteID) + suffix
		rows := transit.TripStopTimes(stopTimes, tripID)
		tracker, ok := eng.getTripSegmentTracker(tripID)
		if len(rows) < 2 || !ok || len(tracker.Shape) < 2 {
			continue
		}
		points := make([]predictions.Point, len(tracker.Shape))
		for i, p := range tracker.Shape {
			points[i] = predictions.Point{Lat: p.ShapePtLat, Lon: p.ShapePtLon}
		}
		stops := make([]predictions.Stop, len(rows))
		cumulative := make([]float64, len(rows))
		total := 0.0
		for i, st := range rows {
			stops[i] = predictions.Stop{ID: st.StopID, Lat: st.StopLat, Lon: st.StopLon, OffsetSec: st.OffsetArrivalTime}
			total += st.OffsetArrivalTime
			cumulative[i] = total
		}
		var turnaround *predictions.Turnaround
		if t, ok := turnarounds[strconv.Itoa(dir)]; ok {
			turnaround = &t
		}
		departures := make(map[string][]int, 3)
		layovers := make(map[string]float64, 3)
		if timetable != nil {
			feederMin := tripDurationMinutes(transit.TripStopTimes(stopTimes, otherDirectionTripID(tripID)))
			days := map[string]models.DaySchedule{calendar.Weekday: timetable.Weekdays, calendar.Saturday: timetable.Saturday, calendar.Sunday: timetable.Sunday}
			for dayType, day := range days {
				for _, d := range transit.Departures(timetable, dayType, dir) {
					departures[dayType] = append(departures[dayType], int(d))
				}
				if layover, ok := scheduledLayoverMinutes(day, feederMin, dir); ok {
					layovers[dayType] = layover
				}
			}
		}
		out = append(out, &liveTrip{
			trip:          predictions.NewTrip(tripID, predictions.NewShapeIndex(points), stops, turnaround),
			routeID:       route.RouteID,
			cumulativeSec: cumulative,
			departures:    departures,
			layoverMin:    layovers,
		})
	}
	return out
}

func rawLiveVehicle(v models.Vehicle, pollInterval time.Duration) predictions.RawVehicle {
	raw := predictions.RawVehicle{
		ID: v.ID, RouteID: v.RouteID, TripID: v.TripID, PrevTripID: v.PrevTripID,
		Lat: v.Latitude, Lon: v.Longitude, PollIntervalMs: pollInterval.Milliseconds(),
	}
	if t, err := time.Parse(time.RFC3339, v.Timestamp); err == nil {
		raw.TimestampMs = t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, v.TripChangedAt); err == nil {
		raw.TripChangedAtMs = t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, v.StationarySince); err == nil {
		raw.StationarySinceMs = t.UnixMilli()
	}
	return raw
}

func (eng *Engine) RecomputeLive(vehicles []models.Vehicle, now time.Time) *liveTable {
	eng.refreshLiveTrips(false)
	eng.liveTrips.RLock()
	trips, rules := eng.liveTrips.byTrip, eng.liveTrips.rules
	eng.liveTrips.RUnlock()
	pollInterval := livePollInterval(eng)
	now = now.In(eng.Location)
	nowMs := now.UnixMilli()
	nowMinutes := float64(now.Hour()*60+now.Minute()) + float64(now.Second())/60
	nowSec := nowMinutes * 60
	dayType := eng.ScheduleDay(now)

	raw := make(map[string][]predictions.RawVehicle)
	byID := make(map[int]models.Vehicle, len(vehicles))
	for _, v := range vehicles {
		if _, ok := trips[v.TripID]; !ok {
			continue
		}
		raw[v.TripID] = append(raw[v.TripID], rawLiveVehicle(v, pollInterval))
		byID[v.ID] = v
	}
	indexed := make(map[string][]predictions.Vehicle, len(raw))
	for tripID, lt := range trips {
		if rv := raw[tripID]; len(rv) > 0 {
			indexed[tripID] = predictions.IndexVehicles(lt.trip, rv, nowMs)
		}
	}

	table := &liveTable{
		AsOf:           now,
		rowsByTrip:     make(map[string][]liveRow, len(trips)),
		rowsByStop:     make(map[int][]liveRow),
		vehiclesByTrip: make(map[string][]liveVehicle, len(indexed)),
	}
	for tripID, lt := range trips {
		own := indexed[tripID]
		var through *predictions.ThroughRun
		if lt.trip.Turnaround != nil && lt.trip.Turnaround.Through {
			through = &predictions.ThroughRun{Turnaround: *lt.trip.Turnaround, Departures: lt.departures[dayType], NowMinutes: nowMinutes, TimetableLayoverMin: lt.layoverMin[dayType]}
			if other, ok := trips[otherDirectionTripID(tripID)]; ok {
				through.FeederDepartures = other.departures[dayType]
				through.Feeder = &predictions.Feeder{Trip: other.trip, Vehicles: indexed[other.trip.ID]}
			}
		}
		departures := lt.departures[dayType]
		tracked := len(own) > 0
		rows := make([][]predictions.Arrival, len(lt.trip.Stops))
		for pos := range lt.trip.Stops {
			etas := predictions.LiveEtasForStop(lt.trip, pos, own, nowMs, through)
			live := make([]predictions.LiveSeconds, len(etas))
			for i, e := range etas {
				live[i] = predictions.LiveSeconds{Seconds: e.Seconds, VehicleID: e.VehicleID, Kind: e.Kind}
			}
			scheduled := predictions.ScheduledArrivals(departures, lt.cumulativeSec[pos], nowSec, liveScheduleHorizon.Seconds(), rules.graceSec)
			rows[pos] = predictions.MergeArrivals(live, scheduled, tracked, liveEntriesPerRow, rules.graceSec)
		}
		marked := predictions.MarkRepeatedNow(rows)
		tripRows := make([]liveRow, len(marked))
		for pos, row := range marked {
			entries := make([]liveEntry, len(row))
			for i, a := range row {
				entries[i] = liveEntry{At: now.Unix() + int64(math.Round(a.Seconds)), Live: a.Live, Soon: a.Soon, VehicleID: a.VehicleID, kind: a.Kind}
			}
			tripRows[pos] = liveRow{TripID: tripID, StopID: lt.trip.Stops[pos].ID, Entries: entries}
			table.rowsByStop[lt.trip.Stops[pos].ID] = append(table.rowsByStop[lt.trip.Stops[pos].ID], tripRows[pos])
		}
		table.rowsByTrip[tripID] = tripRows
		if len(own) > 0 {
			list := make([]liveVehicle, 0, len(own))
			for _, v := range own {
				if !v.OnRoute() {
					continue
				}
				src := byID[v.ID]
				list = append(list, liveVehicle{
					ID: v.ID, TripID: tripID, RouteID: lt.routeID, Latitude: v.Lat, Longitude: v.Lon,
					Heading: math.Round(v.Heading*10) / 10, Speed: math.Max(0, math.Round(src.Speed)), Timestamp: src.Timestamp, Label: src.Label, VehicleType: src.VehicleType,
					Bike: src.BikeAccessible, Wheelchair: src.WheelchairAccessible,
				})
			}
			table.vehiclesByTrip[tripID] = list
		}
	}
	for stopID := range table.rowsByStop {
		rows := table.rowsByStop[stopID]
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].TripID < rows[j].TripID })
	}
	eng.liveState.Lock()
	eng.liveState.table = table
	eng.liveState.Unlock()
	return table
}

func (eng *Engine) currentLiveTable() *liveTable {
	eng.liveState.RLock()
	defer eng.liveState.RUnlock()
	return eng.liveState.table
}

func otherDirectionTripID(tripID string) string {
	if len(tripID) < 2 {
		return tripID
	}
	if tripID[len(tripID)-2:] == models.IncomingSuffix {
		return tripID[:len(tripID)-2] + models.OutgoingSuffix
	}
	return tripID[:len(tripID)-2] + models.IncomingSuffix
}

// The entry closest to slot, live or not, when one is near enough.
func (eng *Engine) ArrivalAt(tripID string, stopID int, slot time.Time) (time.Time, bool) {
	table := eng.currentLiveTable()
	if table == nil {
		return time.Time{}, false
	}
	eng.liveTrips.RLock()
	bestDiff := eng.liveTrips.rules.slotWindow
	eng.liveTrips.RUnlock()
	var best time.Time
	found := false
	for _, row := range table.rowsByTrip[tripID] {
		if row.StopID != stopID {
			continue
		}
		for _, e := range row.Entries {
			at := time.Unix(e.At, 0)
			diff := at.Sub(slot)
			if diff < 0 {
				diff = -diff
			}
			if diff <= bestDiff {
				best, bestDiff, found = at, diff, true
			}
		}
	}
	return best, found
}

type liveTripsState struct {
	sync.RWMutex
	period int64
	day    string
	byTrip map[string]*liveTrip
	rules  liveRules
	// By line number, every route of the network.
	routeIDs map[string]int
}

type liveTableState struct {
	sync.RWMutex
	table *liveTable
}
