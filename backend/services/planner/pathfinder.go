package planner

import (
	"conexiuni-cluj/models"
	"sort"
)

type endpointStop struct {
	stopID   int
	distance float64
}

type leg struct {
	shapeKey    ShapeKey
	tripID      string
	startStopID int
	destStopID  int
	startIdx    int
	destIdx     int
	rideSec     float64
}

type candidate struct {
	legs           []leg
	walkStart      float64
	walkEnd        float64
	walkTransfers  float64
	transitSec     float64
	totalDistance  float64
	originStopID   int
	destEndpointID int
}

func (g *Graph) Plan(req PlanRequest) []candidate {
	if g == nil {
		return nil
	}

	originEndpoints := g.nearbyStops(req.OriginLat, req.OriginLon, endpointWalkRadius, maxEndpointStops)
	destEndpoints := g.nearbyStops(req.DestLat, req.DestLon, endpointWalkRadius, maxEndpointStops)
	if len(originEndpoints) == 0 || len(destEndpoints) == 0 {
		return nil
	}

	directDist := haversineMeters(req.OriginLat, req.OriginLon, req.DestLat, req.DestLon)
	destByID := make(map[int]float64, len(destEndpoints))
	for _, d := range destEndpoints {
		destByID[d.stopID] = d.distance
	}

	var out []candidate
	seen := make(map[string]bool)

	push := func(c candidate) {
		key := candidateKey(c)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, c)
	}

	// 1-leg
	for _, oe := range originEndpoints {
		g.expandDirect(oe, destByID, directDist, push)
	}
	// 2-leg
	for _, oe := range originEndpoints {
		g.expandTwoLeg(oe, destByID, directDist, push)
	}
	// 3-leg (only when 1-leg or 2-leg coverage looks thin)
	if shouldExploreThreeLeg(out) {
		for _, oe := range originEndpoints {
			g.expandThreeLeg(oe, destByID, directDist, push)
		}
	}

	rankAndPrune(&out, directDist)
	if len(out) > maxResults {
		out = out[:maxResults]
	}
	return out
}

func (g *Graph) expandDirect(oe endpointStop, destByID map[int]float64, directDist float64, push func(candidate)) {
	originStop := g.stops[oe.stopID]
	for _, ref := range g.stopShapes[oe.stopID] {
		shape := g.shapes[ref.Key]
		if shape == nil {
			continue
		}
		from := shape.Stops[ref.StopIdx]
		for j := ref.StopIdx + 1; j < len(shape.Stops); j++ {
			ss := shape.Stops[j]
			walkEnd, isDest := destByID[ss.StopID]
			if !isDest {
				continue
			}
			rideSec := ss.OffsetArrivalTime - from.OffsetArrivalTime
			if rideSec <= 0 {
				continue
			}
			busDist := g.legDistance(shape, ref.StopIdx, j)
			c := candidate{
				legs: []leg{{
					shapeKey: ref.Key, tripID: shape.TripIDs[0],
					startStopID: oe.stopID, destStopID: ss.StopID,
					startIdx: ref.StopIdx, destIdx: j, rideSec: rideSec,
				}},
				walkStart:      oe.distance,
				walkEnd:        walkEnd,
				transitSec:     rideSec,
				totalDistance:  oe.distance + busDist + walkEnd,
				originStopID:   oe.stopID,
				destEndpointID: ss.StopID,
			}
			if !c.isReasonable(directDist, originStop) {
				continue
			}
			push(c)
		}
	}
}

func (g *Graph) expandTwoLeg(oe endpointStop, destByID map[int]float64, directDist float64, push func(candidate)) {
	originStop := g.stops[oe.stopID]
	for _, ref1 := range g.stopShapes[oe.stopID] {
		shape1 := g.shapes[ref1.Key]
		if shape1 == nil {
			continue
		}
		from1 := shape1.Stops[ref1.StopIdx]
		for j := ref1.StopIdx + 1; j < len(shape1.Stops); j++ {
			interSS := shape1.Stops[j]
			if g.isDestination(interSS.StopID, destByID) {
				continue // direct cases handled in expandDirect
			}
			interStop, ok := g.stops[interSS.StopID]
			if !ok {
				continue
			}
			// Geographic pruning: don't get off where it won't help reach destination
			if !makesProgress(originStop, interStop, destByID, g) {
				continue
			}
			ride1Sec := interSS.OffsetArrivalTime - from1.OffsetArrivalTime
			if ride1Sec <= 0 {
				continue
			}
			busDist1 := g.legDistance(shape1, ref1.StopIdx, j)

			transferOptions := append([]walkNeighbor{{StopID: interSS.StopID, Distance: 0}}, g.walkPairs[interSS.StopID]...)
			for _, transfer := range transferOptions {
				transferStop, ok := g.stops[transfer.StopID]
				if !ok {
					continue
				}
				if !closerThan(transferStop, interStop, destByID, g) && transfer.StopID != interSS.StopID {
					continue
				}
				for _, ref2 := range g.stopShapes[transfer.StopID] {
					if ref2.Key.RouteID == ref1.Key.RouteID {
						continue
					}
					shape2 := g.shapes[ref2.Key]
					if shape2 == nil {
						continue
					}
					from2 := shape2.Stops[ref2.StopIdx]
					for k := ref2.StopIdx + 1; k < len(shape2.Stops); k++ {
						ss2 := shape2.Stops[k]
						walkEnd, isDest := destByID[ss2.StopID]
						if !isDest {
							continue
						}
						ride2Sec := ss2.OffsetArrivalTime - from2.OffsetArrivalTime
						if ride2Sec <= 0 {
							continue
						}
						busDist2 := g.legDistance(shape2, ref2.StopIdx, k)
						c := candidate{
							legs: []leg{
								{shapeKey: ref1.Key, tripID: shape1.TripIDs[0],
									startStopID: oe.stopID, destStopID: interSS.StopID,
									startIdx: ref1.StopIdx, destIdx: j, rideSec: ride1Sec},
								{shapeKey: ref2.Key, tripID: shape2.TripIDs[0],
									startStopID: transfer.StopID, destStopID: ss2.StopID,
									startIdx: ref2.StopIdx, destIdx: k, rideSec: ride2Sec},
							},
							walkStart:      oe.distance,
							walkEnd:        walkEnd,
							walkTransfers:  transfer.Distance,
							transitSec:     ride1Sec + ride2Sec,
							totalDistance:  oe.distance + busDist1 + transfer.Distance + busDist2 + walkEnd,
							originStopID:   oe.stopID,
							destEndpointID: ss2.StopID,
						}
						if !c.isReasonable(directDist, originStop) {
							continue
						}
						push(c)
						break // first feasible alighting on this shape2 is enough
					}
				}
			}
		}
	}
}

func (g *Graph) expandThreeLeg(oe endpointStop, destByID map[int]float64, directDist float64, push func(candidate)) {
	originStop := g.stops[oe.stopID]
	for _, ref1 := range g.stopShapes[oe.stopID] {
		shape1 := g.shapes[ref1.Key]
		if shape1 == nil {
			continue
		}
		from1 := shape1.Stops[ref1.StopIdx]
		for j := ref1.StopIdx + 1; j < len(shape1.Stops); j++ {
			interSS1 := shape1.Stops[j]
			if g.isDestination(interSS1.StopID, destByID) {
				continue
			}
			interStop1, ok := g.stops[interSS1.StopID]
			if !ok {
				continue
			}
			if !makesProgress(originStop, interStop1, destByID, g) {
				continue
			}
			ride1Sec := interSS1.OffsetArrivalTime - from1.OffsetArrivalTime
			if ride1Sec <= 0 {
				continue
			}
			busDist1 := g.legDistance(shape1, ref1.StopIdx, j)

			transferOpts1 := append([]walkNeighbor{{StopID: interSS1.StopID, Distance: 0}}, g.walkPairs[interSS1.StopID]...)
			for _, t1 := range transferOpts1 {
				ts1, ok := g.stops[t1.StopID]
				if !ok {
					continue
				}
				if !closerThan(ts1, interStop1, destByID, g) && t1.StopID != interSS1.StopID {
					continue
				}
				for _, ref2 := range g.stopShapes[t1.StopID] {
					if ref2.Key.RouteID == ref1.Key.RouteID {
						continue
					}
					shape2 := g.shapes[ref2.Key]
					if shape2 == nil {
						continue
					}
					from2 := shape2.Stops[ref2.StopIdx]
					for k := ref2.StopIdx + 1; k < len(shape2.Stops); k++ {
						interSS2 := shape2.Stops[k]
						if g.isDestination(interSS2.StopID, destByID) {
							continue
						}
						interStop2, ok := g.stops[interSS2.StopID]
						if !ok {
							continue
						}
						if !makesProgress(ts1, interStop2, destByID, g) {
							continue
						}
						ride2Sec := interSS2.OffsetArrivalTime - from2.OffsetArrivalTime
						if ride2Sec <= 0 {
							continue
						}
						busDist2 := g.legDistance(shape2, ref2.StopIdx, k)

						transferOpts2 := append([]walkNeighbor{{StopID: interSS2.StopID, Distance: 0}}, g.walkPairs[interSS2.StopID]...)
						for _, t2 := range transferOpts2 {
							ts2, ok := g.stops[t2.StopID]
							if !ok {
								continue
							}
							if !closerThan(ts2, interStop2, destByID, g) && t2.StopID != interSS2.StopID {
								continue
							}
							for _, ref3 := range g.stopShapes[t2.StopID] {
								if ref3.Key.RouteID == ref2.Key.RouteID || ref3.Key.RouteID == ref1.Key.RouteID {
									continue
								}
								shape3 := g.shapes[ref3.Key]
								if shape3 == nil {
									continue
								}
								from3 := shape3.Stops[ref3.StopIdx]
								for l := ref3.StopIdx + 1; l < len(shape3.Stops); l++ {
									ss3 := shape3.Stops[l]
									walkEnd, isDest := destByID[ss3.StopID]
									if !isDest {
										continue
									}
									ride3Sec := ss3.OffsetArrivalTime - from3.OffsetArrivalTime
									if ride3Sec <= 0 {
										continue
									}
									busDist3 := g.legDistance(shape3, ref3.StopIdx, l)
									c := candidate{
										legs: []leg{
											{shapeKey: ref1.Key, tripID: shape1.TripIDs[0],
												startStopID: oe.stopID, destStopID: interSS1.StopID,
												startIdx: ref1.StopIdx, destIdx: j, rideSec: ride1Sec},
											{shapeKey: ref2.Key, tripID: shape2.TripIDs[0],
												startStopID: t1.StopID, destStopID: interSS2.StopID,
												startIdx: ref2.StopIdx, destIdx: k, rideSec: ride2Sec},
											{shapeKey: ref3.Key, tripID: shape3.TripIDs[0],
												startStopID: t2.StopID, destStopID: ss3.StopID,
												startIdx: ref3.StopIdx, destIdx: l, rideSec: ride3Sec},
										},
										walkStart:      oe.distance,
										walkEnd:        walkEnd,
										walkTransfers:  t1.Distance + t2.Distance,
										transitSec:     ride1Sec + ride2Sec + ride3Sec,
										totalDistance:  oe.distance + busDist1 + t1.Distance + busDist2 + t2.Distance + busDist3 + walkEnd,
										originStopID:   oe.stopID,
										destEndpointID: ss3.StopID,
									}
									if !c.isReasonable(directDist, originStop) {
										continue
									}
									push(c)
									break
								}
							}
						}
					}
				}
			}
		}
	}
}

func (g *Graph) legDistance(shape *ShapePath, startIdx, endIdx int) float64 {
	if shape == nil || startIdx >= endIdx || endIdx >= len(shape.Stops) {
		return 0
	}
	var dist float64
	prev, ok := g.stops[shape.Stops[startIdx].StopID]
	if !ok {
		return 0
	}
	for i := startIdx + 1; i <= endIdx; i++ {
		cur, ok := g.stops[shape.Stops[i].StopID]
		if !ok {
			continue
		}
		dist += haversineMeters(prev.StopLat, prev.StopLon, cur.StopLat, cur.StopLon)
		prev = cur
	}
	return dist
}

func (g *Graph) nearbyStops(lat, lon, radius float64, limit int) []endpointStop {
	type pair struct {
		id   int
		dist float64
	}
	var all []pair
	r2 := radius * radius
	for _, s := range g.stopList {
		// Coarse latitude prefilter
		if (s.StopLat-lat)*(s.StopLat-lat)*111000*111000 > r2 {
			continue
		}
		d := haversineMeters(lat, lon, s.StopLat, s.StopLon)
		if d > radius {
			continue
		}
		all = append(all, pair{s.StopID, d})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].dist < all[j].dist })
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]endpointStop, len(all))
	for i, p := range all {
		out[i] = endpointStop{stopID: p.id, distance: p.dist}
	}
	return out
}

func (g *Graph) isDestination(stopID int, destByID map[int]float64) bool {
	_, ok := destByID[stopID]
	return ok
}

func makesProgress(from, to models.Stop, destByID map[int]float64, g *Graph) bool {
	fromD := minDestDist(from, destByID, g)
	toD := minDestDist(to, destByID, g)
	return toD < fromD-30 // require ≥30 m progress to avoid floating-point ties
}

func closerThan(candidate, baseline models.Stop, destByID map[int]float64, g *Graph) bool {
	return minDestDist(candidate, destByID, g) <= minDestDist(baseline, destByID, g)+50
}

func minDestDist(s models.Stop, destByID map[int]float64, g *Graph) float64 {
	best := -1.0
	for id := range destByID {
		ds, ok := g.stops[id]
		if !ok {
			continue
		}
		d := haversineMeters(s.StopLat, s.StopLon, ds.StopLat, ds.StopLon)
		if best < 0 || d < best {
			best = d
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

func (c candidate) isReasonable(directDist float64, originStop models.Stop) bool {
	totalWalk := c.walkStart + c.walkEnd + c.walkTransfers
	if totalWalk > maxTotalWalkMeters {
		return false
	}
	totalTime := c.transitSec + (totalWalk/walkingSpeedMPerSec)*walkCostFactor + transferPenaltySec*float64(len(c.legs)-1)
	if totalTime > maxTotalTimeSec {
		return false
	}
	if directDist > 800 && c.totalDistance > directDist*3.0 && c.totalDistance > directDist+3500 {
		return false
	}
	return true
}

func candidateKey(c candidate) string {
	var b []byte
	b = append(b, byte(len(c.legs)+'0'))
	for _, l := range c.legs {
		b = append(b, ':')
		b = appendInt(b, l.shapeKey.RouteID)
		b = append(b, '/')
		b = appendInt(b, int(l.shapeKey.Direction))
		b = append(b, '@')
		b = appendInt(b, l.startStopID)
		b = append(b, '>')
		b = appendInt(b, l.destStopID)
	}
	return string(b)
}

func appendInt(dst []byte, v int) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	if v < 0 {
		dst = append(dst, '-')
		v = -v
	}
	var tmp [16]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, tmp[i:]...)
}

func shouldExploreThreeLeg(out []candidate) bool {
	if len(out) < 3 {
		return true
	}
	for _, c := range out {
		if c.totalDistance < 4000 {
			return false // already have a tight option
		}
	}
	return true
}

// rankAndPrune sorts candidates by an estimated trip-cost score and removes
// dominated alternatives, biasing toward fewer legs and shorter end-walks.
func rankAndPrune(out *[]candidate, directDist float64) {
	cs := *out
	sort.SliceStable(cs, func(i, j int) bool {
		return scoreCandidate(cs[i]) < scoreCandidate(cs[j])
	})
	kept := cs[:0]
	for _, c := range cs {
		dominated := false
		for _, k := range kept {
			if dominates(k, c) {
				dominated = true
				break
			}
		}
		if !dominated {
			kept = append(kept, c)
		}
	}
	_ = directDist
	*out = kept
}

func scoreCandidate(c candidate) float64 {
	walkSec := (c.walkStart + c.walkEnd + c.walkTransfers) / walkingSpeedMPerSec
	return c.transitSec + walkSec*walkCostFactor + transferPenaltySec*float64(len(c.legs)-1)
}

func dominates(a, b candidate) bool {
	if len(a.legs) > len(b.legs) {
		return false
	}
	if a.walkEnd > b.walkEnd+50 {
		return false
	}
	if a.totalDistance > b.totalDistance+150 {
		return false
	}
	if a.transitSec > b.transitSec+120 {
		return false
	}
	// Same alighting + same boarding fingerprint: prefer the simpler one.
	if sameShape(a, b) {
		return true
	}
	return scoreCandidate(a)+30 < scoreCandidate(b)
}

func sameShape(a, b candidate) bool {
	if len(a.legs) != len(b.legs) {
		return false
	}
	for i := range a.legs {
		if a.legs[i].shapeKey != b.legs[i].shapeKey {
			return false
		}
		if a.legs[i].startStopID != b.legs[i].startStopID {
			return false
		}
		if a.legs[i].destStopID != b.legs[i].destStopID {
			return false
		}
	}
	return true
}
