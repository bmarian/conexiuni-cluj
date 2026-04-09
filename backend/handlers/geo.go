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
	minDist := math.MaxFloat64
	idx := 0
	for i, s := range shapes {
		d := haversineMeters(stop.StopLat, stop.StopLon, s.ShapePtLat, s.ShapePtLon)
		if d < minDist {
			minDist = d
			idx = i
		}
	}
	return idx
}

func calculateStopOffset(prev, curr models.Stop, shapes []models.Shape) float64 {
	fromIdx := closestShapeIndex(prev, shapes)
	toIdx := closestShapeIndex(curr, shapes)

	if fromIdx > toIdx {
		fromIdx, toIdx = toIdx, fromIdx
	}

	var distMeters float64
	for i := fromIdx; i < toIdx; i++ {
		distMeters += haversineMeters(
			shapes[i].ShapePtLat, shapes[i].ShapePtLon,
			shapes[i+1].ShapePtLat, shapes[i+1].ShapePtLon,
		)
	}

	return distMeters / averageBusSpeedMps
}
