package live

import "conexiuni-cluj/models"

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

func (eng *Engine) passengerShape(shape []models.Shape) []models.Shape {
	tracker, ok := eng.getTripSegmentTracker(shape[0].ShapeID)
	if !ok || len(tracker.Shape) != len(shape) {
		return shape
	}
	first, last := 0, len(tracker.Stops)-1
	for first < last && eng.StopHidden(tracker.Stops[first].StopID) {
		first++
	}
	for last > first && eng.StopHidden(tracker.Stops[last].StopID) {
		last--
	}
	start, end := 0, len(shape)
	if first > 0 {
		start = tracker.Stops[first].ShapeIdx
	}
	if last < len(tracker.Stops)-1 {
		end = tracker.Stops[last].ShapeIdx + 1
	}
	return shape[start:end]
}
