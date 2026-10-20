// Package geo measures along the ground and along shapes.
package geo

import (
	"conexiuni-cluj/models"
	"math"
)

const (
	earthRadiusMeters  = 6_371_000.0
	averageBusSpeedMps = 25.0 / 3.6
)

// Haversine is the great-circle distance in meters.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

func ClosestShapeIndex(lat, lon float64, shapes []models.Shape) int {
	minDist := math.MaxFloat64
	idx := 0
	for i, s := range shapes {
		d := Haversine(lat, lon, s.ShapePtLat, s.ShapePtLon)
		if d < minDist {
			minDist = d
			idx = i
		}
	}
	return idx
}

// ShapeDistance is the meters along shapes from one point to a later one.
func ShapeDistance(shapes []models.Shape, fromIdx, toIdx int) float64 {
	if len(shapes) == 0 || fromIdx < 0 || toIdx < 0 || fromIdx >= toIdx || fromIdx >= len(shapes) {
		return 0
	}
	if toIdx >= len(shapes) {
		toIdx = len(shapes) - 1
	}
	var distMeters float64
	for i := fromIdx; i < toIdx; i++ {
		distMeters += Haversine(
			shapes[i].ShapePtLat, shapes[i].ShapePtLon,
			shapes[i+1].ShapePtLat, shapes[i+1].ShapePtLon,
		)
	}
	return distMeters
}

// StopOffset is the seconds between two stops along shapes at bus pace, for a feed without times.
func StopOffset(prev, curr models.Stop, shapes []models.Shape) float64 {
	fromIdx := ClosestShapeIndex(prev.StopLat, prev.StopLon, shapes)
	toIdx := ClosestShapeIndex(curr.StopLat, curr.StopLon, shapes)

	if fromIdx > toIdx {
		fromIdx, toIdx = toIdx, fromIdx
	}

	return ShapeDistance(shapes, fromIdx, toIdx) / averageBusSpeedMps
}
