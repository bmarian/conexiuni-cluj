package handlers

import (
	"conexiuni-cluj/models"
	ctpcj "conexiuni-cluj/services/ctp-cj"
	"conexiuni-cluj/services/tranzy"
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
				var rows []models.StopTime
				for _, st := range stopTimes {
					if strings.HasSuffix(st.TripID, dirKey.suffix) {
						rows = append(rows, st)
					}
				}
				if len(rows) == 0 {
					continue
				}
				sort.Slice(rows, func(i, j int) bool { return rows[i].StopSequence < rows[j].StopSequence })
				stopRefs := make([]models.PlaydateStopRef, 0, len(rows))
				for _, st := range rows {
					stopRefs = append(stopRefs, models.PlaydateStopRef{StopID: st.StopID, StopName: st.StopHeadsign})
				}
				directions[dirKey.name] = models.PlaydateDirection{
					Headsign: headsignByTripID[rows[0].TripID],
					Stops:    stopRefs,
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
