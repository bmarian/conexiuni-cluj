package handlers

import (
	"conexiuni-cluj/models"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"sort"
	"strings"
)

const shapePolylinePrecision = 6

func encodePolyline(points []models.Shape, precision int) string {
	factor := math.Pow(10, float64(precision))
	var b strings.Builder
	prevLat, prevLon := 0, 0
	for _, p := range points {
		lat := int(math.Round(p.ShapePtLat * factor))
		lon := int(math.Round(p.ShapePtLon * factor))
		writePolylineValue(&b, lat-prevLat)
		writePolylineValue(&b, lon-prevLon)
		prevLat, prevLon = lat, lon
	}
	return b.String()
}

func writePolylineValue(b *strings.Builder, v int) {
	shifted := v << 1
	if v < 0 {
		shifted = ^shifted
	}
	for shifted >= 0x20 {
		b.WriteByte(byte((0x20 | (shifted & 0x1f)) + 63))
		shifted >>= 5
	}
	b.WriteByte(byte(shifted + 63))
}

func shapePolylines(points []models.Shape) map[string]string {
	byID := make(map[string][]models.Shape)
	for _, p := range points {
		byID[p.ShapeID] = append(byID[p.ShapeID], p)
	}
	out := make(map[string]string, len(byID))
	for id, shape := range byID {
		sort.SliceStable(shape, func(i, j int) bool { return shape[i].ShapePtSequence < shape[j].ShapePtSequence })
		out[id] = encodePolyline(shape, shapePolylinePrecision)
	}
	return out
}

func shapeHash(polyline string) string {
	sum := sha256.Sum256([]byte(polyline))
	return hex.EncodeToString(sum[:6])
}
