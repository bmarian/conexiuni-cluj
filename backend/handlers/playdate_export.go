package handlers

import (
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// GetPlaydateExport assembles the full offline snapshot for the Playdate app
func GetPlaydateExport(tranzyClient *tranzy.Client, ctpCjClient *ctpcj.Client, cacheTimes models.CacheTimes) (*models.PlaydateExport, error) {
	allRoutes, err := GetRoutes(tranzyClient, cacheTimes.TranzyCacheShelfLife, RouteFilter{})
	if err != nil {
		return nil, err
	}
	if Availability.IsReady() {
		filtered := make([]models.Route, 0, len(allRoutes))
		for _, r := range allRoutes {
			if Availability.RouteHasTimetable(r.RouteShortName) {
				filtered = append(filtered, r)
			}
		}
		allRoutes = filtered
	}

	allStops, err := GetStops(tranzyClient, cacheTimes.TranzyCacheShelfLife, StopFilter{})
	if err != nil {
		return nil, err
	}
	if Availability.IsReady() {
		filtered := make([]models.Stop, 0, len(allStops))
		for _, s := range allStops {
			if Availability.StopHasBuses(s.StopID) {
				filtered = append(filtered, s)
			}
		}
		allStops = filtered
	}

	stops := make([]models.PlaydateStop, 0, len(allStops))
	for _, s := range allStops {
		stops = append(stops, models.PlaydateStop{
			StopID:   s.StopID,
			StopName: s.StopName,
			StopLat:  s.StopLat,
			StopLon:  s.StopLon,
		})
	}

	routes := buildPlaydateRoutesParallel(tranzyClient, ctpCjClient, cacheTimes, allRoutes)

	return &models.PlaydateExport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Routes:      routes,
		Stops:       stops,
	}, nil
}

func buildPlaydateRoutesParallel(
	tranzyClient *tranzy.Client,
	ctpCjClient *ctpcj.Client,
	cacheTimes models.CacheTimes,
	routes []models.Route,
) []models.PlaydateRoute {
	type slot struct {
		ok    bool
		route models.PlaydateRoute
	}
	slots := make([]slot, len(routes))

	var wg sync.WaitGroup
	for i, route := range routes {
		wg.Add(1)
		go func(i int, route models.Route) {
			defer wg.Done()
			rsn := route.RouteShortName

			var (
				trips     []models.Trip
				stopTimes []models.StopTime
				timetable *models.Timetable
				tErr, stErr, ttErr error
				inner sync.WaitGroup
			)
			inner.Add(3)
			go func() {
				defer inner.Done()
				trips, tErr = GetTrips(tranzyClient, cacheTimes.TranzyCacheShelfLife, TripFilter{RouteID: &route.RouteID})
			}()
			go func() {
				defer inner.Done()
				stopTimes, stErr = GetStopTimes(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn})
			}()
			go func() {
				defer inner.Done()
				timetable, ttErr = GetTimetable(ctpCjClient, tranzyClient, cacheTimes, rsn)
			}()
			inner.Wait()

			if tErr != nil || stErr != nil || ttErr != nil || timetable == nil {
				return
			}

			// One set of cumulative offsets per hour of day (0-23): segment
			// travel-time profiles vary by time of day, and this snapshot is
			// synced once and browsed offline for up to a day, so a single
			// "as of sync time" offset would drift wrong as the day goes on.
			// Sequential per route (not further parallelized across hours)
			// to bound total concurrent DB queries across all 105 routes.
			today := time.Now().In(tranzyClient.Location())
			hourlyStopTimes := make(map[int][]models.StopTime, 24)
			for hour := 0; hour < 24; hour++ {
				refTime := time.Date(today.Year(), today.Month(), today.Day(), hour, 0, 0, 0, today.Location())
				hourStopTimes, err := GetStopTimesAt(tranzyClient, cacheTimes, StopTimeFilter{RouteShortName: &rsn}, refTime)
				if err != nil {
					continue
				}
				hourlyStopTimes[hour] = hourStopTimes
			}
			if len(timetable.Weekdays.Entries) == 0 &&
				len(timetable.Saturday.Entries) == 0 &&
				len(timetable.Sunday.Entries) == 0 {
				return
			}

			headsignByTripID := make(map[string]string, len(trips))
			for _, t := range trips {
				headsignByTripID[NormalizeTripID(t.TripID)] = t.TripHeadsign
			}

			directions := map[string]models.PlaydateDirection{}
			for _, dirKey := range []struct {
				suffix, name string
			}{{OUTGOING_SUFFIX, "out"}, {INCOMING_SUFFIX, "in"}} {
				rows := filterAndSortStopTimesBySuffix(stopTimes, dirKey.suffix)
				if len(rows) == 0 {
					continue
				}
				stopRefs := make([]models.PlaydateStopRef, 0, len(rows))
				for _, st := range rows {
					stopRefs = append(stopRefs, models.PlaydateStopRef{StopID: st.StopID, StopName: st.StopHeadsign})
				}

				hourlyOffsets := make(map[int][]int, len(hourlyStopTimes))
				for hour, hourRows := range hourlyStopTimes {
					hourFiltered := filterAndSortStopTimesBySuffix(hourRows, dirKey.suffix)
					if len(hourFiltered) != len(rows) {
						// Shouldn't happen (same trips exist regardless of ref_hour),
						// but skip rather than risk misaligning against stopRefs.
						continue
					}
					offsets := make([]int, len(hourFiltered))
					cumulative := 0.0
					for i, st := range hourFiltered {
						cumulative += st.OffsetArrivalTime
						offsets[i] = int(math.Round(cumulative))
					}
					hourlyOffsets[hour] = offsets
				}

				directions[dirKey.name] = models.PlaydateDirection{
					Headsign:      headsignByTripID[rows[0].TripID],
					Stops:         stopRefs,
					HourlyOffsets: hourlyOffsets,
				}
			}
			if len(directions) == 0 {
				return
			}

			slots[i] = slot{
				ok: true,
				route: models.PlaydateRoute{
					RouteID:        route.RouteID,
					RouteShortName: rsn,
					RouteLongName:  route.RouteLongName,
					RouteColor:     route.RouteColor,
					Directions:     directions,
					Timetable:      *timetable,
				},
			}
		}(i, route)
	}
	wg.Wait()

	out := make([]models.PlaydateRoute, 0, len(slots))
	for _, s := range slots {
		if s.ok {
			out = append(out, s.route)
		}
	}
	return out
}

// filterAndSortStopTimesBySuffix returns the rows for one direction
// (trip_id suffix "_0"/"_1"), sorted by stop_sequence.
func filterAndSortStopTimesBySuffix(rows []models.StopTime, suffix string) []models.StopTime {
	var out []models.StopTime
	for _, st := range rows {
		if strings.HasSuffix(st.TripID, suffix) {
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StopSequence < out[j].StopSequence })
	return out
}
