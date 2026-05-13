package handlers

import (
	"conexiuni-cluj/models"
	"math"
)

const (
	earthRadiusMeters  = 6_371_000.0
	averageBusSpeedMps = 25.0 / 3.6 // 25 km/h in m/s
)

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

func closestShapeIndex(stop models.Stop, shapes []models.Shape) int {
	return closestShapeIndexLatLon(stop.StopLat, stop.StopLon, shapes)
}

func closestShapeIndexLatLon(lat, lon float64, shapes []models.Shape) int {
	minDist := math.MaxFloat64
	idx := 0
	for i, s := range shapes {
		d := haversineMeters(lat, lon, s.ShapePtLat, s.ShapePtLon)
		if d < minDist {
			minDist = d
			idx = i
		}
	}
	return idx
}

func shapeDistanceMeters(shapes []models.Shape, fromIdx, toIdx int) float64 {
	if len(shapes) == 0 || fromIdx < 0 || toIdx < 0 || fromIdx >= toIdx || fromIdx >= len(shapes) {
		return 0
	}
	if toIdx >= len(shapes) {
		toIdx = len(shapes) - 1
	}
	var distMeters float64
	for i := fromIdx; i < toIdx; i++ {
		distMeters += haversineMeters(
			shapes[i].ShapePtLat, shapes[i].ShapePtLon,
			shapes[i+1].ShapePtLat, shapes[i+1].ShapePtLon,
		)
	}
	return distMeters
}

func calculateStopOffset(prev, curr models.Stop, shapes []models.Shape) float64 {
	fromIdx := closestShapeIndex(prev, shapes)
	toIdx := closestShapeIndex(curr, shapes)

	if fromIdx > toIdx {
		fromIdx, toIdx = toIdx, fromIdx
	}

	return shapeDistanceMeters(shapes, fromIdx, toIdx) / averageBusSpeedMps
}
