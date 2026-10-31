package live

import (
	"conexiuni-cluj/models"
	"conexiuni-cluj/predictions"
	"math"
)

func (eng *Engine) PassengerShapes(points []models.Shape) []models.Shape {
	if len(eng.HiddenStops()) == 0 {
		return points
	}
	out := make([]models.Shape, 0, len(points))
	for from := 0; from < len(points); {
		to := from
		for to < len(points) && points[to].ShapeID == points[from].ShapeID {
			to++
		}
		out = append(out, eng.passengerShape(points[from:to])...)
		from = to
	}
	return out
}

// The first and last stops of a trip that are not hidden, by position.
func visibleEnds(count int, hidden func(pos int) bool) (first, last int) {
	first, last = 0, count-1
	for first < last && hidden(first) {
		first++
	}
	for last > first && hidden(last) {
		last--
	}
	return first, last
}

func (eng *Engine) passengerShape(shape []models.Shape) []models.Shape {
	tracker, ok := eng.getTripSegmentTracker(shape[0].ShapeID)
	if !ok || len(tracker.Shape) != len(shape) {
		return shape
	}
	first, last := visibleEnds(len(tracker.Stops), func(pos int) bool { return eng.StopHidden(tracker.Stops[pos].StopID) })
	start, end := 0, len(shape)
	if first > 0 {
		start = tracker.Stops[first].ShapeIdx
	}
	if last < len(tracker.Stops)-1 {
		end = tracker.Stops[last].ShapeIdx + 1
	}
	return shape[start:end]
}

// A bus at a hidden stand past either end of the drawn line is not shown; its stop rows stay.
func onDrawnLine(trip *predictions.Trip, shapeIdx int, hidden func(stopID int) bool) bool {
	first, last := visibleEnds(len(trip.Stops), func(pos int) bool { return hidden(trip.Stops[pos].ID) })
	if first > 0 && shapeIdx < trip.Positions[first] && trip.Shape.Distance(shapeIdx, trip.Positions[first]) > stopReach(trip, first-1, first) {
		return false
	}
	if last < len(trip.Stops)-1 && shapeIdx > trip.Positions[last] && trip.Shape.Distance(trip.Positions[last], shapeIdx) > stopReach(trip, last, last+1) {
		return false
	}
	return true
}

// How far a bus still counts as at the visible stop of two neighbours: never closer to the hidden one.
func stopReach(trip *predictions.Trip, from, to int) float64 {
	return math.Min(segmentStopProximityMeters, trip.Shape.Distance(trip.Positions[from], trip.Positions[to])/2)
}
