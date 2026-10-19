package handlers

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/router"
	"conexiuni-cluj/sources"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const (
	// Must match the clients' walking pace.
	plannerWalkSpeed   = 80.0 / 60
	plannerMaxWalk     = 15 * time.Minute
	plannerMaxTransfer = 30 * time.Minute
	plannerTicketCost  = 15 * time.Minute
)

func OSMPath() string { return filepath.Join("services", "osm", "romania-latest.osm.pbf") }

// osmLocate reads towns and streets from the planner's OSM extract.
func osmLocate(points []sources.Point) ([]string, []string, error) {
	f, err := os.Open(OSMPath())
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	at := make([]router.Point, len(points))
	for i, p := range points {
		at[i] = router.Point{Lat: p.Lat, Lon: p.Lon}
	}
	towns, streets, err := router.Localities(f, at)
	// Reading the country extract peaks far above what is kept.
	debug.FreeOSMemory()
	return towns, streets, err
}

// nil until the walk graph is read.
func (city *City) plannerPlaces() *router.Places {
	city.planner.Lock()
	defer city.planner.Unlock()
	return city.planner.places
}

// plannerNetwork is the timetable around at's service day, rebuilt when the learned offsets roll over.
func (city *City) plannerNetwork(at time.Time) (*router.Network, error) {
	city.planner.Lock()
	if city.planner.streets == nil {
		if err := city.readPlannerStreets(); err != nil {
			city.planner.Unlock()
			return nil, err
		}
	}

	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())
	key := day.Format("2006-01-02")
	if period := ProfileSnapshotPeriod(); period != city.planner.period || city.planner.networks == nil {
		city.planner.period, city.planner.networks = period, make(map[string]*router.Network)
	}
	if n, ok := city.planner.networks[key]; ok {
		city.planner.Unlock()
		return n, nil
	}
	streets := city.planner.streets
	if !livePlanDay(key, time.Now().In(at.Location())) {
		// Built outside the lock: one far-off date must not hold up everyone's plans.
		city.planner.Unlock()
		return city.buildPlannerNetwork(streets, day)
	}
	defer city.planner.Unlock()
	n, err := city.buildPlannerNetwork(streets, day)
	if err != nil {
		return nil, err
	}
	city.planner.networks[key] = n
	return n, nil
}

// readPlannerStreets reads the walk graph again when the served stops changed. The caller holds the lock.
func (city *City) readPlannerStreets() error {
	stops, err := city.servedStops()
	if err != nil {
		return err
	}
	h := fnv.New64a()
	for _, s := range stops {
		fmt.Fprint(h, s.StopID, s.StopLat, s.StopLon)
	}
	stopsKey := h.Sum64()
	if city.planner.streets != nil && city.planner.stopsKey == stopsKey {
		return nil
	}
	streets, places, err := city.buildPlannerStreets(stops)
	if err != nil {
		return err
	}
	city.planner.streets, city.planner.places, city.planner.stopsKey, city.planner.networks = streets, places, stopsKey, nil
	return nil
}

func (city *City) buildPlannerNetwork(streets *router.Streets, day time.Time) (*router.Network, error) {
	started := time.Now()
	patterns, err := city.plannerPatterns(day)
	if err != nil {
		return nil, err
	}
	n := streets.Network(patterns)
	runs := 0
	for _, p := range patterns {
		runs += len(p.Trips)
	}
	city.log("planner: %d patterns, %d runs around %s in %s", len(patterns), runs, day.Format("2006-01-02"), time.Since(started).Round(time.Millisecond))
	return n, nil
}

// Only the days clients keep re-planning on live data stay built; any other date is one request.
func livePlanDay(key string, now time.Time) bool {
	return key == now.Format("2006-01-02") || key == now.Add(liveScheduleHorizon).Format("2006-01-02")
}

// servedStops are the stops some trip calls at; Tranzy also lists stops of other cities.
func (city *City) servedStops() ([]models.Stop, error) {
	stops, err := city.GetStops(StopFilter{})
	if err != nil {
		return nil, err
	}
	calls, err := city.getTripStops()
	if err != nil {
		return nil, err
	}
	served := make(map[int]bool, len(stops))
	for _, c := range calls {
		served[c.StopID] = true
	}
	out := make([]models.Stop, 0, len(served))
	for _, s := range stops {
		if served[s.StopID] {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, plannerUnavailableError{fmt.Errorf("planner: no served stops yet")}
	}
	slices.SortFunc(out, func(a, b models.Stop) int { return a.StopID - b.StopID })
	return out, nil
}

func (city *City) buildPlannerStreets(stops []models.Stop) (*router.Streets, *router.Places, error) {
	return city.buildStreets(stops, router.Options{
		WalkSpeed:     plannerWalkSpeed,
		MaxWalk:       plannerMaxWalk,
		MaxTransfer:   plannerMaxTransfer,
		TransferSlack: time.Duration(city.learnLiveRules().graceSec * float64(time.Second)),
		TicketCost:    plannerTicketCost,
	})
}

func (city *City) buildStreets(stops []models.Stop, opt router.Options) (*router.Streets, *router.Places, error) {
	started := time.Now()
	box := router.BBox{MinLat: math.Inf(1), MinLon: math.Inf(1), MaxLat: math.Inf(-1), MaxLon: math.Inf(-1)}
	routerStops := make([]router.Stop, len(stops))
	for i, s := range stops {
		routerStops[i] = router.Stop{ID: s.StopID, Lat: s.StopLat, Lon: s.StopLon}
		box.MinLat, box.MaxLat = math.Min(box.MinLat, s.StopLat), math.Max(box.MaxLat, s.StopLat)
		box.MinLon, box.MaxLon = math.Min(box.MinLon, s.StopLon), math.Max(box.MaxLon, s.StopLon)
	}
	f, err := os.Open(OSMPath())
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	walk, places, err := router.Load(f, box.Grow(opt.WalkSpeed*max(opt.MaxWalk, opt.MaxTransfer).Seconds()))
	if err != nil {
		return nil, nil, err
	}
	streets := router.NewStreets(walk, routerStops, opt)
	// Reading the country extract peaks far above what the graph keeps.
	debug.FreeOSMemory()
	city.log("planner: walk graph of %d nodes for %d stops, %d places, in %s, transfer slack %s", walk.Nodes(), len(stops), places.Count(), time.Since(started).Round(time.Millisecond), opt.TransferSlack)
	return streets, places, nil
}

// Runs of the day before, the day and the day after, timed like the stop pages.
func (city *City) plannerPatterns(day time.Time) ([]router.PatternInput, error) {
	routes, err := city.GetRoutes(RouteFilter{})
	if err != nil {
		return nil, err
	}
	var out []router.PatternInput
	for _, route := range routes {
		rsn := route.RouteShortName
		base, routeID, err := city.getBaseStopTimes(StopTimeFilter{RouteShortName: &rsn})
		if err != nil || routeID == 0 {
			continue
		}
		timetable, _ := city.GetTimetable(rsn)
		if timetable == nil {
			continue
		}
		cumulative := make(map[string]map[string][]float64)
		offsetsAt := func(departure time.Time, tripID string) []float64 {
			key := city.profileDay(departure) + "|" + strconv.Itoa(segmentBucketStartMin(departure))
			byTrip, ok := cumulative[key]
			if !ok {
				timed := city.applySegmentProfilesToStopTimes(base, routeID, departure)
				byTrip = make(map[string][]float64, 2)
				for _, suffix := range []string{OUTGOING_SUFFIX, INCOMING_SUFFIX} {
					id := strconv.Itoa(routeID) + suffix
					total := 0.0
					for _, st := range sortedTrip(timed, id) {
						total += st.OffsetArrivalTime
						byTrip[id] = append(byTrip[id], total)
					}
				}
				cumulative[key] = byTrip
			}
			return byTrip[tripID]
		}
		for dir, suffix := range []string{OUTGOING_SUFFIX, INCOMING_SUFFIX} {
			tripID := strconv.Itoa(routeID) + suffix
			rows := sortedTrip(base, tripID)
			if len(rows) < 2 {
				continue
			}
			// The city ticket does not cover the metropolitan (M) lines.
			p := router.PatternInput{RouteID: routeID, Key: tripID, Stops: make([]int, len(rows)), OwnTicket: strings.HasPrefix(rsn, "M")}
			for i, st := range rows {
				p.Stops[i] = st.StopID
			}
			for shift := -1; shift <= 1; shift++ {
				date := day.AddDate(0, 0, shift)
				for _, m := range scheduleDepartureMinutes(timetable, city.scheduleDay(date), dir) {
					departure := time.Date(date.Year(), date.Month(), date.Day(), 0, int(m), 0, 0, date.Location())
					offsets := offsetsAt(departure, tripID)
					if len(offsets) != len(rows) {
						continue
					}
					run := make([]int64, len(rows))
					for i, o := range offsets {
						run[i] = departure.Unix() + int64(math.Round(o))
					}
					p.Trips = append(p.Trips, run)
				}
			}
			if len(p.Trips) > 0 {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

type planStop struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type planLine struct {
	ShortName string           `json:"short_name"`
	LongName  string           `json:"long_name"`
	Color     string           `json:"color"`
	Type      models.RouteType `json:"type"`
}

type planRide struct {
	RouteID   int    `json:"route_id"`
	TripID    string `json:"trip_id"`
	Depart    int64  `json:"depart"`
	Arrive    int64  `json:"arrive"`
	Live      bool   `json:"live,omitempty"`
	VehicleID int    `json:"vehicle_id,omitempty"`
}

// Unix seconds.
type planRun struct {
	Depart int64      `json:"depart"`
	Arrive int64      `json:"arrive"`
	Rides  []planRide `json:"rides"`
}

// Stops are 0 at the origin and the destination; Lines are the lines of the buses the runs take.
type planLeg struct {
	Kind    string  `json:"kind"`
	From    int     `json:"from"`
	To      int     `json:"to"`
	Passed  []int   `json:"passed,omitempty"`
	Meters  float64 `json:"meters,omitempty"`
	Seconds int64   `json:"seconds"`
	Path    string  `json:"path"`
	Lines   []int   `json:"lines,omitempty"`
}

type planOption struct {
	Key       string    `json:"key"`
	Transfers int       `json:"transfers"`
	WalkM     float64   `json:"walk_m"`
	Legs      []planLeg `json:"legs"`
	Runs      []planRun `json:"runs"`
}

// Seconds until the timings change; 0 when they don't.
type planResponse struct {
	Options   []planOption        `json:"options"`
	Stops     map[string]planStop `json:"stops"`
	Lines     map[string]planLine `json:"lines"`
	RefreshIn int64               `json:"refresh_in"`
}

// Runs after every ingest, the only writer of stops, so plans never re-read them.
func (city *City) WarmPlanner() {
	city.planner.Lock()
	err := city.readPlannerStreets()
	city.planner.Unlock()
	if err == nil {
		_, err = city.plannerNetwork(time.Now().In(city.Location))
	}
	if err != nil {
		city.log("planner: %v", err)
	}
}

func (city *City) handlePlan(c fiber.Ctx) error {
	fromLat, fromLng, toLat, toLng, err := parsePlanCoords(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	when, arriveBy, err := parsePlanTime(c, city.Location)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if when.IsZero() {
		when = time.Now().In(city.Location)
	}
	release, ok := acquirePlanSlot(ClientHashFromLocals(c))
	if !ok {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	defer release()
	asked := when
	last := c.Query("last") == "true"
	n, err := city.plannerNetwork(when)
	if err == nil && last {
		when, arriveBy = time.Unix(n.QuietEnd(when.Unix()), 0).In(city.Location), true
		n, err = city.plannerNetwork(when)
	}
	if err != nil {
		city.log("plan: %v", err)
		var starting plannerUnavailableError
		if errors.As(err, &starting) {
			c.Set("Retry-After", "10")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "planner still starting", "retry_after_seconds": 10})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "routing failed"})
	}

	from, to := router.Point{Lat: fromLat, Lon: fromLng}, router.Point{Lat: toLat, Lon: toLng}
	resp := planResponse{Options: []planOption{}, Stops: map[string]planStop{}, Lines: map[string]planLine{}}
	if !n.Covers(from) || !n.Covers(to) {
		c.Set("Cache-Control", revalidateCacheControl)
		return c.JSON(resp)
	}
	q := router.Query{From: from, To: to, Time: when, ArriveBy: arriveBy, Runs: liveEntriesPerRow, Last: last}
	var table *liveTable
	if !arriveBy {
		table, q.Live = city.plannerLiveAt(when)
	}
	options := n.Plan(q)
	if last && len(options) == 1 && options[0].Rides == 0 {
		// No bus at all: the walk, from now.
		options[0].Runs[0] = router.Run{Depart: asked.Unix(), Arrive: asked.Unix() + options[0].Legs[0].Secs}
	}

	if err := city.describePlan(&resp, options); err != nil {
		city.log("plan: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "routing failed"})
	}

	if table != nil {
		var firsts []int64
		for _, o := range options {
			if o.Rides > 0 {
				firsts = append(firsts, o.Runs[0].Rides[0].Depart)
			}
		}
		resp.RefreshIn = city.refreshIn(table, firsts)
	}
	c.Set("Cache-Control", revalidateCacheControl)
	return c.JSON(resp)
}

// plannerLiveAt is the live table when a plan from `when` can use it: fresh, and within its horizon.
func (city *City) plannerLiveAt(when time.Time) (*liveTable, router.LiveSource) {
	if city.hub == nil || !when.Before(time.Now().Add(liveScheduleHorizon)) {
		return nil, nil
	}
	table, err := city.hub.current()
	if err != nil {
		city.log("plan: live: %v", err)
		return nil, nil
	}
	return table, plannerLive(table)
}

// Never sooner than the next vehicle poll.
func (city *City) refreshIn(table *liveTable, firsts []int64) int64 {
	if table == nil || len(firsts) == 0 {
		return 0
	}
	wait := slices.Min(firsts) - time.Now().Unix()
	if interval := livePollInterval(city); interval > 0 {
		wait = max(wait, int64(interval.Seconds()))
	}
	return max(1, wait)
}

// plannerLive reads the stop pages' own rows, so the planner shows the buses they show.
func plannerLive(table *liveTable) router.LiveSource {
	return func(key string, stop int) []router.Arrival {
		for _, row := range table.rowsByTrip[key] {
			if row.StopID != stop {
				continue
			}
			out := make([]router.Arrival, len(row.Entries))
			for i, e := range row.Entries {
				out[i] = router.Arrival{At: e.At, Live: e.Live, VehicleID: e.VehicleID}
			}
			return out
		}
		return nil
	}
}

func (city *City) describePlan(resp *planResponse, options []router.Option) error {
	stops, err := city.GetStops(StopFilter{})
	if err != nil {
		return err
	}
	routes, err := city.GetRoutes(RouteFilter{})
	if err != nil {
		return err
	}
	stopByID := make(map[int]models.Stop, len(stops))
	for _, s := range stops {
		stopByID[s.StopID] = s
	}
	routeByID := make(map[int]models.Route, len(routes))
	for _, r := range routes {
		routeByID[r.RouteID] = r
	}
	addStop := func(id int) {
		if s, ok := stopByID[id]; ok && id != 0 {
			resp.Stops[strconv.Itoa(id)] = planStop{Name: s.StopName, Lat: s.StopLat, Lon: s.StopLon}
		}
	}

	for _, o := range options {
		po := planOption{Transfers: max(0, o.Rides-1), WalkM: o.WalkMeters}
		for _, r := range o.Runs {
			pr := planRun{Depart: r.Depart, Arrive: r.Arrive, Rides: []planRide{}}
			for _, ride := range r.Rides {
				pr.Rides = append(pr.Rides, planRide{RouteID: ride.RouteID, TripID: ride.Key, Depart: ride.Depart, Arrive: ride.Arrive, Live: ride.Live, VehicleID: ride.VehicleID})
			}
			po.Runs = append(po.Runs, pr)
		}
		var key []string
		ride := 0
		for _, l := range o.Legs {
			leg := planLeg{Kind: "walk", From: l.FromStop, To: l.ToStop, Meters: l.Meters, Seconds: l.Secs}
			addStop(l.FromStop)
			addStop(l.ToStop)
			if !l.Ride {
				leg.Path = pointsPolyline(l.Path)
				po.Legs = append(po.Legs, leg)
				continue
			}
			leg.Kind, leg.Passed = "ride", l.Passed
			for _, id := range l.Passed {
				addStop(id)
			}
			for _, r := range po.Runs {
				if id := r.Rides[ride].RouteID; !slices.Contains(leg.Lines, id) {
					leg.Lines = append(leg.Lines, id)
				}
			}
			for _, id := range leg.Lines {
				if r, ok := routeByID[id]; ok {
					color := r.RouteColor
					if color == "" {
						color = models.ResolveRouteDisplayColor(r.RouteShortName)
					}
					resp.Lines[strconv.Itoa(id)] = planLine{ShortName: r.RouteShortName, LongName: r.RouteLongName, Color: color, Type: r.RouteType}
				}
			}
			first := po.Runs[0].Rides[ride]
			leg.Seconds = first.Arrive - first.Depart
			leg.Path = city.ridePolyline(first.TripID, l, stopByID)
			key = append(key, fmt.Sprintf("%d>%d", l.FromStop, l.ToStop))
			po.Legs = append(po.Legs, leg)
			ride++
		}
		po.Key = strings.Join(key, "|")
		if o.Rides == 0 {
			po.Key = "walk"
		}
		resp.Options = append(resp.Options, po)
	}
	return nil
}

// ridePolyline is the line's shape between the two stops, or straight through its stops without one.
func (city *City) ridePolyline(tripID string, l router.Leg, stopByID map[int]models.Stop) string {
	if tracker, ok := city.getTripSegmentTracker(tripID); ok {
		for i, s := range tracker.Stops {
			if s.StopID != l.FromStop {
				continue
			}
			for _, e := range tracker.Stops[i+1:] {
				if e.StopID == l.ToStop && e.ShapeIdx >= s.ShapeIdx {
					return encodePolyline(tracker.Shape[s.ShapeIdx:e.ShapeIdx+1], 5)
				}
			}
		}
	}
	var points []models.Shape
	for _, id := range append(append([]int{l.FromStop}, l.Passed...), l.ToStop) {
		s := stopByID[id]
		points = append(points, models.Shape{ShapePtLat: s.StopLat, ShapePtLon: s.StopLon})
	}
	return encodePolyline(points, 5)
}

func pointsPolyline(path []router.Point) string {
	points := make([]models.Shape, len(path))
	for i, p := range path {
		points[i] = models.Shape{ShapePtLat: p.Lat, ShapePtLon: p.Lon}
	}
	return encodePolyline(points, 5)
}

type plannerUnavailableError struct {
	err error
}

func (e plannerUnavailableError) Error() string { return e.err.Error() }

func (e plannerUnavailableError) Unwrap() error { return e.err }

func parsePlanCoords(c fiber.Ctx) (fromLat, fromLng, toLat, toLng float64, err error) {
	fromLatS := c.Query("from_lat")
	fromLngS := c.Query("from_lng")
	toLatS := c.Query("to_lat")
	toLngS := c.Query("to_lng")
	if fromLatS == "" || fromLngS == "" || toLatS == "" || toLngS == "" {
		return 0, 0, 0, 0, errors.New("from_lat, from_lng, to_lat, to_lng are required")
	}
	if fromLat, err = strconv.ParseFloat(fromLatS, 64); err != nil {
		return 0, 0, 0, 0, errors.New("invalid from_lat")
	}
	if fromLng, err = strconv.ParseFloat(fromLngS, 64); err != nil {
		return 0, 0, 0, 0, errors.New("invalid from_lng")
	}
	if toLat, err = strconv.ParseFloat(toLatS, 64); err != nil {
		return 0, 0, 0, 0, errors.New("invalid to_lat")
	}
	if toLng, err = strconv.ParseFloat(toLngS, 64); err != nil {
		return 0, 0, 0, 0, errors.New("invalid to_lng")
	}
	if !isValidLat(fromLat) || !isValidLng(fromLng) || !isValidLat(toLat) || !isValidLng(toLng) {
		return 0, 0, 0, 0, errors.New("coordinates out of valid range")
	}
	return fromLat, fromLng, toLat, toLng, nil
}

// time is RFC3339 or local "2006-01-02T15:04"; empty means now.
func parsePlanTime(c fiber.Ctx, loc *time.Location) (time.Time, bool, error) {
	timeStr := strings.TrimSpace(c.Query("time"))
	arriveBy := strings.EqualFold(strings.TrimSpace(c.Query("arrive_by")), "true") ||
		c.Query("arrive_by") == "1"

	if timeStr == "" {
		return time.Time{}, arriveBy, nil
	}

	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t.In(loc), arriveBy, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", timeStr, loc); err == nil {
		return t, arriveBy, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", timeStr, loc); err == nil {
		return t, arriveBy, nil
	}
	return time.Time{}, false, errors.New("invalid time (expected RFC3339 or 2006-01-02T15:04)")
}

func isValidLat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -90 && v <= 90
}

func isValidLng(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -180 && v <= 180
}
