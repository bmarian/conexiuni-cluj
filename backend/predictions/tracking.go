package predictions

import (
	"math"
	"sort"
)

const (
	TerminusRadiusMeters = 200.0
	// GPS and feed limits.
	maxOffRouteMeters       = 150.0
	vehicleGracePeriodMs    = 10 * 60_000
	layoverHideMs           = 30 * 60_000
	headingLookahead        = 3
	liveEtaMaxPositionAgeMs = 4 * 60_000
	liveEtaMaxClockSkewMs   = 60_000
	// No bus in Cluj goes faster; the learner rejects the same.
	MaxBusSpeedKmh = 70.0
	// Matching a bus to its timetable run: within this, and within less than half the
	// headway so that it is not nearer another run.
	maxRunMatchMin       = 8.0
	runMatchHeadwayShare = 0.4
	pairedSlotMarginMin  = 3.0
	// Measured on the Oct 2026 recordings; each terminus replaces them as its stands are
	// seen (handlers/turnaround.go). Buses wait for a slot about as long as the timetable
	// plans, never under ten minutes, never over twenty.
	DefaultStandSec = 90.0
	minSlotWaitMin  = 10.0
	maxSlotWaitMin  = 20.0
	layoverSlackMin = 5.0
	// Further out, a bus still on the other direction predicts no better than the
	// timetable (scored at 5, 8, 12 and 18 minutes on the same recordings).
	FeederHorizonMs = 12 * 60_000.0
)

type Point struct{ Lat, Lon float64 }

type ShapeIndex struct {
	Points     []Point
	Cumulative []float64
}

func NewShapeIndex(points []Point) *ShapeIndex {
	cum := make([]float64, len(points))
	for i := 1; i < len(points); i++ {
		cum[i] = cum[i-1] + HaversineMeters(points[i-1].Lat, points[i-1].Lon, points[i].Lat, points[i].Lon)
	}
	return &ShapeIndex{Points: points, Cumulative: cum}
}

func (s *ShapeIndex) Closest(lat, lon float64) (int, float64) {
	idx, best := -1, math.Inf(1)
	for i, p := range s.Points {
		d := HaversineMeters(p.Lat, p.Lon, lat, lon)
		if d < best {
			best, idx = d, i
		}
	}
	return idx, best
}

func (s *ShapeIndex) Distance(from, to int) float64 {
	if from < 0 || to < 0 || from > to || to >= len(s.Cumulative) {
		return 0
	}
	return s.Cumulative[to] - s.Cumulative[from]
}

// Stop is one stop of a trip; OffsetSec is the ride into it from the previous stop, and
// on the first stop the learned schedule delay.
type Stop struct {
	ID        int
	Lat, Lon  float64
	OffsetSec float64
}

// Turnaround is how buses ending the other direction here continue as this trip: after
// the stand, or at the next timetable slot when that is later and within the waits seen
// here. Learned per terminus; the defaults and the timetable stand in until then.
type Turnaround struct {
	Through  bool
	StandSec float64
	// The longest a bus is known to wait here; zero until learned.
	MaxWaitMin float64
}

type Trip struct {
	ID    string
	Shape *ShapeIndex
	Stops []Stop
	// Closest shape index of each stop, and the same held monotone along the trip.
	StopIdx      []int
	Positions    []int
	FirstStopIdx int
	// The learned pace over the whole trip, for a bus between stops nobody has timed.
	SpeedMps   float64
	Turnaround *Turnaround
}

func NewTrip(id string, shape *ShapeIndex, stops []Stop, turnaround *Turnaround) *Trip {
	t := &Trip{ID: id, Shape: shape, Stops: stops, Turnaround: turnaround, FirstStopIdx: math.MaxInt32}
	t.StopIdx = make([]int, len(stops))
	t.Positions = make([]int, len(stops))
	last := 0
	for i, st := range stops {
		idx := -1
		if st.Lat != 0 && st.Lon != 0 && shape != nil {
			idx, _ = shape.Closest(st.Lat, st.Lon)
		}
		t.StopIdx[i] = idx
		if idx < 0 {
			idx = last
		}
		if idx > last {
			last = idx
		}
		t.Positions[i] = last
	}
	if len(stops) > 1 && t.StopIdx[1] >= 0 {
		t.FirstStopIdx = t.StopIdx[1]
	}
	seconds := 0.0
	for _, st := range stops[1:] {
		seconds += st.OffsetSec
	}
	if meters := t.Shape.Distance(t.Positions[0], t.Positions[len(stops)-1]); meters > 0 && seconds > 0 {
		t.SpeedMps = meters / seconds
	}
	return t
}

func (t *Trip) StopPos(stopID int) int {
	for i, st := range t.Stops {
		if st.ID == stopID {
			return i
		}
	}
	return -1
}

type RawVehicle struct {
	ID                int
	RouteID           int
	TripID            string
	PrevTripID        string
	Lat, Lon          float64
	TimestampMs       int64
	TripChangedAtMs   int64
	StationarySinceMs int64
	// How often positions arrive: a fix is at most this old on average.
	PollIntervalMs int64
}

type Vehicle struct {
	RawVehicle
	ShapeIdx        int
	OffRouteMeters  float64
	AtStartTerminus bool
	DwellMs         int64
	Heading         float64
}

func (v *Vehicle) OnRoute() bool { return v.ShapeIdx >= 0 && v.OffRouteMeters <= maxOffRouteMeters }

func isTracked(v RawVehicle, nowMs int64) bool {
	if v.Lat <= 0 || v.Lon <= 0 || v.TimestampMs == 0 || nowMs-v.TimestampMs > vehicleGracePeriodMs {
		return false
	}
	return dwellMs(v, nowMs) < layoverHideMs
}

func dwellMs(v RawVehicle, nowMs int64) int64 {
	if v.StationarySinceMs == 0 || nowMs < v.StationarySinceMs {
		return 0
	}
	return nowMs - v.StationarySinceMs
}

func isFreshForLiveEta(v RawVehicle, nowMs int64) bool {
	age := nowMs - v.TimestampMs
	return v.TimestampMs != 0 && age >= -liveEtaMaxClockSkewMs && age <= liveEtaMaxPositionAgeMs
}

func heading(lat, lon float64, shape *ShapeIndex, idx int) float64 {
	pts := shape.Points
	if idx < 0 || len(pts) == 0 {
		return 0
	}
	targetIdx := idx + headingLookahead
	if targetIdx > len(pts)-1 {
		targetIdx = len(pts) - 1
	}
	target := pts[targetIdx]
	if targetIdx == idx || HaversineMeters(lat, lon, target.Lat, target.Lon) < 2 {
		if idx > 0 {
			return Bearing(pts[idx-1].Lat, pts[idx-1].Lon, pts[idx].Lat, pts[idx].Lon)
		}
		if len(pts) > 1 {
			return Bearing(pts[0].Lat, pts[0].Lon, pts[1].Lat, pts[1].Lon)
		}
	}
	return Bearing(lat, lon, target.Lat, target.Lon)
}

// IndexVehicles snaps the trip's reported vehicles onto its shape.
func IndexVehicles(trip *Trip, raw []RawVehicle, nowMs int64) []Vehicle {
	if trip == nil || trip.Shape == nil || len(trip.Shape.Points) == 0 {
		return nil
	}
	start := trip.Shape.Points[0]
	out := make([]Vehicle, 0, len(raw))
	for _, v := range raw {
		if !isTracked(v, nowMs) {
			continue
		}
		idx, dist := trip.Shape.Closest(v.Lat, v.Lon)
		out = append(out, Vehicle{
			RawVehicle:      v,
			ShapeIdx:        idx,
			OffRouteMeters:  dist,
			AtStartTerminus: HaversineMeters(v.Lat, v.Lon, start.Lat, start.Lon) <= TerminusRadiusMeters,
			DwellMs:         dwellMs(v, nowMs),
			Heading:         heading(v.Lat, v.Lon, trip.Shape, idx),
		})
	}
	return out
}

// Only a bus that has left is past the trip's first stop: parked buses sit short of it.
func canDriveLiveEta(v *Vehicle, nowMs int64, firstStopIdx int) bool {
	waiting := v.AtStartTerminus && v.ShapeIdx <= firstStopIdx
	return v.OnRoute() && !waiting && isFreshForLiveEta(v.RawVehicle, nowMs)
}

// Learned segment times for the rest of the ride, with the current segment's remaining
// share; the fix age is credited since the bus has driven on since it reported.
func profileAwareEtaSeconds(trip *Trip, v *Vehicle, targetPos int, nowMs int64) (float64, bool) {
	if targetPos <= 0 || targetPos >= len(trip.Stops) {
		return 0, false
	}
	prevPos := -1
	for i := 0; i < targetPos; i++ {
		if trip.Positions[i] <= v.ShapeIdx {
			prevPos = i
		} else {
			break
		}
	}
	if prevPos < 0 {
		return 0, false
	}
	nextPos := prevPos + 1
	if nextPos > targetPos {
		return 0, false
	}
	segStart, segEnd := trip.Positions[prevPos], trip.Positions[nextPos]
	segMeters := trip.Shape.Distance(segStart, segEnd)
	from := v.ShapeIdx
	if segStart > from {
		from = segStart
	}
	remaining := trip.Shape.Distance(from, segEnd)
	share := 0.0
	if segMeters > 0 {
		share = math.Min(1, math.Max(0, remaining/segMeters))
	}
	seconds := 0.0
	for pos := nextPos; pos <= targetPos; pos++ {
		f := 1.0
		if pos == nextPos {
			f = share
		}
		seconds += trip.Stops[pos].OffsetSec * f
	}
	if seconds <= 0 {
		return 0, false
	}
	fixAge := math.Min(float64(v.PollIntervalMs)/1000, math.Max(0, float64(nowMs-v.TimestampMs)/1000))
	return math.Max(0, seconds-fixAge), true
}

// VehicleEtaSeconds is the ride from the vehicle to the stop at targetPos: the learned
// segment times, or the trip's learned pace where it is between stops nobody has timed;
// never faster than a bus goes.
func VehicleEtaSeconds(trip *Trip, v *Vehicle, targetPos int, nowMs int64) float64 {
	meters := trip.Shape.Distance(v.ShapeIdx, trip.StopIdx[targetPos])
	est, ok := profileAwareEtaSeconds(trip, v, targetPos, nowMs)
	if !ok && trip.SpeedMps > 0 {
		est = meters / trip.SpeedMps
	}
	return math.Max(est, meters/(MaxBusSpeedKmh/3.6))
}

type EtaKind int

const (
	EtaOnTrip EtaKind = iota
	EtaStanding
	EtaFeeder
)

type Eta struct {
	VehicleID int
	Seconds   float64
	Kind      EtaKind
}

type Feeder struct {
	Trip     *Trip
	Vehicles []Vehicle
}

// ThroughRun describes a trip whose buses arrive on the other direction and continue.
type ThroughRun struct {
	Turnaround       Turnaround
	Departures       []int
	FeederDepartures []int
	NowMinutes       float64
	// What the day's timetable plans between arriving and leaving again.
	TimetableLayoverMin float64
	Feeder              *Feeder
}

// A slot further off than this is a hole in the timetable, not a wait.
func (t *ThroughRun) maxWaitMin() float64 {
	if t.Turnaround.MaxWaitMin > 0 {
		return t.Turnaround.MaxWaitMin
	}
	return math.Min(maxSlotWaitMin, math.Max(minSlotWaitMin, t.TimetableLayoverMin+layoverSlackMin))
}

func rideSecondsFromStart(stops []Stop, targetPos int) float64 {
	total := 0.0
	for pos := 1; pos <= targetPos && pos < len(stops); pos++ {
		total += stops[pos].OffsetSec
	}
	return total
}

func routeOf(tripID string) string {
	for i := 0; i < len(tripID); i++ {
		if tripID[i] == '_' {
			return tripID[:i]
		}
	}
	return tripID
}

func cameFromOtherDirection(v *Vehicle) bool {
	return v.PrevTripID != "" && v.PrevTripID != v.TripID && routeOf(v.PrevTripID) == routeOf(v.TripID)
}

// A bus flipped onto this trip at its first stop, within the longest stand known here.
func standingAtStart(v *Vehicle, nowMs int64, firstStopIdx int, through *ThroughRun) bool {
	maxStandMs := (through.Turnaround.StandSec + through.maxWaitMin()*60) * 1000
	return v.AtStartTerminus && v.ShapeIdx <= firstStopIdx && v.OnRoute() && isFreshForLiveEta(v.RawVehicle, nowMs) &&
		v.TripChangedAtMs != 0 && float64(nowMs-v.TripChangedAtMs) <= maxStandMs && cameFromOtherDirection(v)
}

// MatchRunDeparture is the timetable departure a run belongs to, unless it sits too far
// between two.
func MatchRunDeparture(departures []int, impliedMin float64) (int, bool) {
	sorted := append([]int(nil), departures...)
	sort.Ints(sorted)
	best, bestDiff := -1, math.Inf(1)
	for i, d := range sorted {
		diff := math.Abs(MinutesUntil(float64(d), impliedMin))
		if diff < bestDiff {
			bestDiff, best = diff, i
		}
	}
	if best < 0 {
		return 0, false
	}
	gap := math.Inf(1)
	if best > 0 {
		gap = float64(sorted[best] - sorted[best-1])
	}
	if best+1 < len(sorted) {
		gap = math.Min(gap, float64(sorted[best+1]-sorted[best]))
	}
	if bestDiff <= math.Min(maxRunMatchMin, runMatchHeadwayShare*gap) {
		return sorted[best], true
	}
	return 0, false
}

// The first slot after the feeder run's scheduled arrival: in the past for a late bus.
func pairedSlotMinutes(through *ThroughRun, arrivalMin, feederRideMin float64) (float64, bool) {
	if len(through.FeederDepartures) == 0 {
		return 0, false
	}
	matched, ok := MatchRunDeparture(through.FeederDepartures, arrivalMin-feederRideMin)
	if !ok {
		return 0, false
	}
	scheduledArrival := float64(matched) + feederRideMin
	slot, found := 0.0, false
	for _, d := range through.Departures {
		diff := MinutesUntil(float64(d), scheduledArrival)
		if diff >= -pairedSlotMarginMin && (!found || diff < slot) {
			slot, found = diff, true
		}
	}
	if !found {
		return 0, false
	}
	return scheduledArrival + slot, true
}

// ProjectDeparture is when a bus arriving at arrivalMs leaves: after its stand, or at
// its slot when that comes later and is within the waits seen here. A standing bus past
// that moment takes the next slot.
func ProjectDeparture(arrivalMs float64, through *ThroughRun, nowMs int64, standing bool, feederRideMin float64, hasFeederRide bool) (float64, bool) {
	now := float64(nowMs)
	minute := func(ms float64) float64 { return through.NowMinutes + (ms-now)/60_000 }
	ready := arrivalMs + through.Turnaround.StandSec*1000
	departure := ready
	maxWait := through.maxWaitMin()
	slot, found := 0.0, false
	if hasFeederRide {
		slot, found = pairedSlotMinutes(through, minute(arrivalMs), feederRideMin)
	}
	if !found {
		slot, found = nextSlotMinutes(through.Departures, minute(ready))
	}
	if found {
		if wait := MinutesUntil(slot, minute(ready)); wait > 0 && wait <= maxWait {
			departure = ready + wait*60_000
		}
	}
	if standing && now > departure {
		next, found := nextSlotMinutes(through.Departures, through.NowMinutes)
		if !found || MinutesUntil(next, through.NowMinutes) > maxWait {
			return 0, false
		}
		departure = now + MinutesUntil(next, through.NowMinutes)*60_000
	}
	return departure, true
}

// The first slot at or after fromMin.
func nextSlotMinutes(departures []int, fromMin float64) (float64, bool) {
	next, found := 0.0, false
	for _, d := range departures {
		diff := MinutesUntil(float64(d), fromMin)
		if diff >= 0 && (!found || diff < next) {
			next, found = diff, true
		}
	}
	return fromMin + next, found
}

func toEta(v *Vehicle, seconds float64, kind EtaKind) Eta {
	return Eta{VehicleID: v.ID, Seconds: math.Max(0, seconds), Kind: kind}
}

// LiveEtasForStop lists every live estimate for the stop at targetPos, soonest first,
// including the buses still to arrive from the other direction when the trip runs through.
func LiveEtasForStop(trip *Trip, targetPos int, vehicles []Vehicle, nowMs int64, through *ThroughRun) []Eta {
	if targetPos < 0 || targetPos >= len(trip.Stops) || trip.StopIdx[targetPos] < 0 {
		return nil
	}
	stopIdx := trip.StopIdx[targetPos]
	var etas []Eta
	for i := range vehicles {
		v := &vehicles[i]
		if v.ShapeIdx <= stopIdx && canDriveLiveEta(v, nowMs, trip.FirstStopIdx) {
			etas = append(etas, toEta(v, VehicleEtaSeconds(trip, v, targetPos, nowMs), EtaOnTrip))
		}
	}
	if through != nil && through.Turnaround.Through && len(trip.Stops) > 0 {
		ride := rideSecondsFromStart(trip.Stops, targetPos)
		feederRideMin, hasFeederRide := 0.0, false
		if through.Feeder != nil && through.Feeder.Trip != nil && len(through.Feeder.Trip.Stops) > 0 {
			feederRideMin = rideSecondsFromStart(through.Feeder.Trip.Stops, len(through.Feeder.Trip.Stops)-1) / 60
			hasFeederRide = true
		}
		for i := range vehicles {
			v := &vehicles[i]
			if !standingAtStart(v, nowMs, trip.FirstStopIdx, through) {
				continue
			}
			if d, ok := ProjectDeparture(float64(v.TripChangedAtMs), through, nowMs, true, feederRideMin, hasFeederRide); ok {
				etas = append(etas, toEta(v, math.Max(0, (d-float64(nowMs))/1000)+ride, EtaStanding))
			}
		}
		if hasFeederRide {
			ft := through.Feeder.Trip
			lastPos := len(ft.Stops) - 1
			last := ft.Stops[lastPos]
			if last.Lat != 0 && last.Lon != 0 && ft.StopIdx[lastPos] >= 0 {
				lastIdx := ft.StopIdx[lastPos]
				for i := range through.Feeder.Vehicles {
					v := &through.Feeder.Vehicles[i]
					if v.ShapeIdx > lastIdx || !canDriveLiveEta(v, nowMs, ft.FirstStopIdx) {
						continue
					}
					toEnd := VehicleEtaSeconds(ft, v, lastPos, nowMs)
					d, ok := ProjectDeparture(float64(nowMs)+toEnd*1000, through, nowMs, false, feederRideMin, true)
					if !ok || d-float64(nowMs) > FeederHorizonMs {
						continue
					}
					etas = append(etas, toEta(v, math.Max(0, (d-float64(nowMs))/1000)+ride, EtaFeeder))
				}
			}
		}
	}
	sort.SliceStable(etas, func(i, j int) bool { return etas[i].Seconds < etas[j].Seconds })
	return etas
}

func HaversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := (lat2-lat1)*math.Pi/180, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// Bearing from the first point to the second, in degrees clockwise from north.
func Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dl := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dl) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dl)
	deg := math.Atan2(y, x) * 180 / math.Pi
	return math.Mod(deg+360, 360)
}
