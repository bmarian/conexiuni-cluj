import type {ShapeInfo, Stop, StopInfo, StopTime} from '@/types/tranzy.ts'

export type RouteLeg = {
  routes: ShapeInfo[]
  startStop: StopInfo | Stop
  destStop: StopInfo | Stop
  tripIds: string[]
}

export type PlannedRoute = {
  legs: RouteLeg[]
  isDirect: boolean
  totalDistance: number
}

export type DirectRoute = {
  route: ShapeInfo
  startStop: StopInfo | Stop
  destStop: StopInfo | Stop
  tripId: string
}

export const getRouteIdFromTripId = (tripId: string): number | null => {
  const [routeIdPart] = String(tripId).split('_')
  const routeId = Number(routeIdPart)
  return Number.isFinite(routeId) ? routeId : null
}

export const getTripIdForRouteAtStop = (outgoingTripIds: string[], incomingTripIds: string[], routeId: number | string): string | undefined => {
  const wantedRouteId = Number(routeId)
  if (!Number.isFinite(wantedRouteId)) return undefined
  return [...(outgoingTripIds || []), ...(incomingTripIds || [])]
    .find((id) => getRouteIdFromTripId(id) === wantedRouteId)
}

export const getTimeOffsetToStop = (stopTimes: StopTime[], tripId: string, stopId: number): number => {
  let timeOffset = 0
  for (const st of stopTimes) {
    if (st.trip_id !== tripId) continue
    if (st.stop_id === stopId) break
    timeOffset += Math.ceil(st.offset_arrival_time / 60)
  }
  return timeOffset
}

export const getShapeStopTimes = (shapeInfo: ShapeInfo | null | undefined): StopTime[] => {
  return shapeInfo?.stop_times ?? shapeInfo?.stop_time ?? []
}

const calculateLegDistance = (tripStopTimes: StopTime[], startST: StopTime, destST: StopTime): number => {
  let dist = 0
  const legSTs = tripStopTimes
    .filter(st => st.stop_sequence >= startST.stop_sequence && st.stop_sequence <= destST.stop_sequence)
    .sort((a, b) => a.stop_sequence - b.stop_sequence)

  for (let i = 0; i < legSTs.length - 1; i++) {
    const current = legSTs[i]
    const next = legSTs[i + 1]
    if (current && next) {
      dist += haversineMeters(current.stop_lat, current.stop_lon, next.stop_lat, next.stop_lon)
    }
  }
  return dist
}

const getStopSequenceKey = (tripStopTimes: StopTime[], startST: StopTime, destST: StopTime): string => {
  return tripStopTimes
    .filter(st => st.stop_sequence >= startST.stop_sequence && st.stop_sequence <= destST.stop_sequence)
    .sort((a, b) => a.stop_sequence - b.stop_sequence)
    .map(st => st.stop_id)
    .join(',')
}

const groupPlannedRoutes = (routes: PlannedRoute[]): PlannedRoute[] => {
  const groups = new Map<string, PlannedRoute>()

  for (const r of routes) {
    const key = (r.isDirect ? 'D' : 'C') + r.legs.map(leg => {
      const route = leg.routes[0]
      const tripId = leg.tripIds[0]
      if (!route || !tripId) return ''
      const stopTimes = getShapeStopTimes(route)
      const tripStopTimes = stopTimes.filter(st => st.trip_id === tripId)
      const startST = tripStopTimes.find(st => st.stop_id === leg.startStop.stop_id)
      const destST = tripStopTimes.find(st => st.stop_id === leg.destStop.stop_id)
      if (!startST || !destST) return ''
      return `|${getStopSequenceKey(tripStopTimes, startST, destST)}`
    }).join('')

    if (groups.has(key)) {
      const existing = groups.get(key)!
      for (let i = 0; i < r.legs.length; i++) {
        const rLeg = r.legs[i]
        const eLeg = existing.legs[i]
        if (!rLeg || !eLeg) continue
        for (let j = 0; j < rLeg.routes.length; j++) {
          const route = rLeg.routes[j]
          const tripId = rLeg.tripIds[j]
          if (route && tripId && !eLeg.routes.some(rt => rt.route_id === route.route_id)) {
            eLeg.routes.push(route)
            eLeg.tripIds.push(tripId)
          }
        }
      }
      existing.totalDistance = Math.min(existing.totalDistance, r.totalDistance)
    } else {
      groups.set(key, JSON.parse(JSON.stringify(r)))
    }
  }

  return Array.from(groups.values())
}

const filterFatRoutes = (routes: PlannedRoute[], userLat: number, userLon: number, destLat: number, destLon: number): PlannedRoute[] => {
  const withMetrics = routes.map(r => {
    const lastLeg = r.legs[r.legs.length - 1]!
    const destDistance = haversineMeters(lastLeg.destStop.stop_lat, lastLeg.destStop.stop_lon, destLat, destLon)

    let transitDuration = 0
    for (const leg of r.legs) {
      const route = leg.routes[0]
      const tripId = leg.tripIds[0]
      if (route && tripId) {
        const stopTimes = getShapeStopTimes(route)
        const tripStopTimes = stopTimes.filter(st => st.trip_id === tripId)
        const startST = tripStopTimes.find(st => st.stop_id === leg.startStop.stop_id)
        const destST = tripStopTimes.find(st => st.stop_id === leg.destStop.stop_id)
        if (startST && destST) {
          transitDuration += (destST.offset_arrival_time - startST.offset_arrival_time)
        }
      }
    }
    // 10m penalty for change to strongly discourage unnecessary transfers
    transitDuration += (r.legs.length - 1) * 600

    return {
      route: r,
      destDistance,
      totalDistance: r.totalDistance,
      duration: transitDuration,
      changes: r.legs.length - 1
    }
  })

  // Sort: fewer changes first, then closer to destination, then shorter distance
  withMetrics.sort((a, b) => a.changes - b.changes || a.destDistance - b.destDistance || a.totalDistance - b.totalDistance)

  const directDist = haversineMeters(userLat, userLon, destLat, destLon)

  const kept: typeof withMetrics = []
  for (const cand of withMetrics) {
    // Basic detour prune: if total distance is more than 2.5x direct distance, it's a fat route
    if (directDist > 500 && cand.totalDistance > directDist * 2.5 && cand.totalDistance > directDist + 2000) continue

    const dominated = kept.some(existing => {
      // Existing dominates if it's better or equal in ALL metrics
      // Using tolerances to favor simpler (fewer changes) or much shorter routes
      const betterOrEqual =
        existing.destDistance <= cand.destDistance + 100 && // 100m tolerance for walking
        existing.totalDistance <= cand.totalDistance + 200 && // 200m tolerance for transit
        existing.duration <= cand.duration + 180 && // 3m tolerance for duration
        existing.changes <= cand.changes

      // Aggressive pruning: if existing is much shorter and faster, it dominates even if cand is closer
      const muchBetter =
        existing.totalDistance < cand.totalDistance * 0.6 &&
        existing.duration < cand.duration * 0.7 &&
        existing.changes <= cand.changes

      return betterOrEqual || (muchBetter && existing.destDistance < cand.destDistance + 500)
    })

    if (!dominated) {
      kept.push(cand)
    }
  }

  return kept.map(k => k.route)
}

export const findRoutes = (
  startStops: StopInfo[],
  destStops: StopInfo[],
  allStops: Stop[],
  userLat: number,
  userLon: number,
  destLat: number,
  destLon: number
): PlannedRoute[] => {
  const directLegs = findDirectRoutes(startStops, destStops)
  const directDestStopIds = new Set(directLegs.map(dl => dl.destStop.stop_id))

  const results: PlannedRoute[] = []

  // Add Direct Routes
  for (const dl of directLegs) {
    const walkStart = haversineMeters(userLat, userLon, dl.startStop.stop_lat, dl.startStop.stop_lon)
    const walkEnd = haversineMeters(dl.destStop.stop_lat, dl.destStop.stop_lon, destLat, destLon)

    const stopTimes = getShapeStopTimes(dl.route)
    const tripStopTimes = stopTimes.filter(st => st.trip_id === dl.tripId)
    const startST = tripStopTimes.find(st => st.stop_id === dl.startStop.stop_id)!
    const destST = tripStopTimes.find(st => st.stop_id === dl.destStop.stop_id)!
    const busDist = calculateLegDistance(tripStopTimes, startST, destST)

    results.push({
      isDirect: true,
      legs: [{
        routes: [dl.route],
        tripIds: [dl.tripId],
        startStop: dl.startStop,
        destStop: dl.destStop
      }],
      totalDistance: walkStart + busDist + walkEnd
    })
  }

  // Use a map to group connecting routes by path during initial search
  // Key: stopSeq1 | transferStopId | stopSeq2 | destStopId
  const connectingPaths = new Map<string, PlannedRoute>()

  // Try to find connecting routes
  for (const startStop of startStops) {
    for (const shape1 of startStop.shapes_info) {
      const stopTimes1 = getShapeStopTimes(shape1)
      const tripGroups1 = groupStopTimesByTrip(stopTimes1)

      for (const [tripId1, tripStopTimes1] of tripGroups1) {
        const startST1 = tripStopTimes1.find(st => st.stop_id === startStop.stop_id)
        if (!startST1) continue

        const afterStart1 = tripStopTimes1.filter(st => st.stop_sequence > startST1.stop_sequence)

        for (const interST of afterStart1) {
          const interStop = allStops.find(s => s.stop_id === interST.stop_id)
          if (!interStop) continue

          const nearbyInterStops = allStops.filter(s =>
            s.stop_id === interST.stop_id ||
            haversineMeters(interST.stop_lat, interST.stop_lon, s.stop_lat, s.stop_lon) <= 250
          )

          for (const destStop of destStops) {
            if (directDestStopIds.has(destStop.stop_id)) continue

            for (const shape2 of destStop.shapes_info) {
              if (shape1.route_id === shape2.route_id) continue

              const stopTimes2 = getShapeStopTimes(shape2)
              const tripGroups2 = groupStopTimesByTrip(stopTimes2)

              for (const transferStop of nearbyInterStops) {
                const walkTransfer = transferStop.stop_id === interST.stop_id ? 0 : haversineMeters(interST.stop_lat, interST.stop_lon, transferStop.stop_lat, transferStop.stop_lon)

                for (const [tripId2, tripStopTimes2] of tripGroups2) {
                  const transferST2 = tripStopTimes2.find(st => st.stop_id === transferStop.stop_id)
                  const destST2 = tripStopTimes2.find(st => st.stop_id === destStop.stop_id)

                  if (transferST2 && destST2 && transferST2.stop_sequence < destST2.stop_sequence) {
                    // Feasibility: trip2 must arrive after trip1 (plus buffer)
                    const minTransferSeconds = 60 + (walkTransfer * 1.5)
                    if (transferST2.offset_arrival_time >= interST.offset_arrival_time + minTransferSeconds) {
                      const stopSeq1 = getStopSequenceKey(tripStopTimes1, startST1, interST)
                      const stopSeq2 = getStopSequenceKey(tripStopTimes2, transferST2, destST2)
                      const pathKey = `${stopSeq1}|${interStop.stop_id}|${transferStop.stop_id}|${stopSeq2}|${destStop.stop_id}`

                      if (!connectingPaths.has(pathKey)) {
                        const walkStart = haversineMeters(userLat, userLon, startStop.stop_lat, startStop.stop_lon)
                        const busDist1 = calculateLegDistance(tripStopTimes1, startST1, interST)
                        const busDist2 = calculateLegDistance(tripStopTimes2, transferST2, destST2)
                        const walkEnd = haversineMeters(destStop.stop_lat, destStop.stop_lon, destLat, destLon)

                        connectingPaths.set(pathKey, {
                          isDirect: false,
                          legs: [
                            {routes: [shape1], startStop, destStop: interStop, tripIds: [tripId1]},
                            {routes: [shape2], startStop: transferStop, destStop, tripIds: [tripId2]}
                          ],
                          totalDistance: walkStart + busDist1 + walkTransfer + busDist2 + walkEnd
                        })
                      }
                      // Once we found one feasible trip combination for this (shape1, inter, shape2) pair,
                      // we can move to the next shape2 to keep things manageable.
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
  }

  results.push(...Array.from(connectingPaths.values()))

  const filtered = filterFatRoutes(results, userLat, userLon, destLat, destLon)
  const grouped = groupPlannedRoutes(filtered)
  return grouped.sort((a, b) => a.totalDistance - b.totalDistance)
}

const groupStopTimesByTrip = (stopTimes: StopTime[]): Map<string, StopTime[]> => {
  const tripGroups = new Map<string, StopTime[]>()
  for (const st of stopTimes) {
    if (!tripGroups.has(st.trip_id)) {
      tripGroups.set(st.trip_id, [])
    }
    tripGroups.get(st.trip_id)!.push(st)
  }
  return tripGroups
}

import {haversineMeters} from "@/utils/geo.ts";

export const findDirectRoutes = (
  startStops: StopInfo[],
  destStops: StopInfo[]
): DirectRoute[] => {
  const directRoutes: DirectRoute[] = []
  const seenRoutes = new Set<string>()

  for (const startStop of startStops) {
    for (const shape of startStop.shapes_info) {
      const routeId = String(shape.route_id)
      if (seenRoutes.has(routeId)) continue

      const stopTimes = getShapeStopTimes(shape)

      // Group stopTimes by trip_id once per shape
      const tripGroups = new Map<string, StopTime[]>()
      for (const st of stopTimes) {
        if (!tripGroups.has(st.trip_id)) {
          tripGroups.set(st.trip_id, [])
        }
        tripGroups.get(st.trip_id)!.push(st)
      }

      // Pre-calculate which trips of this shape pass through startStop
      const tripsPassingStart = new Map<string, StopTime>()
      for (const [tripId, tripStopTimes] of tripGroups) {
        const st = tripStopTimes.find(s => s.stop_id === startStop.stop_id)
        if (st) tripsPassingStart.set(tripId, st)
      }

      let foundForThisShape = false
      // Prioritize destination proximity: check each destStop in order
      for (const destStop of destStops) {
        // For this destination, check if any trip starts at startStop and ends here
        for (const [tripId, startST] of tripsPassingStart) {
          const tripStopTimes = tripGroups.get(tripId)!
          const destST = tripStopTimes.find(st => st.stop_id === destStop.stop_id)

          if (destST && startST.stop_sequence < destST.stop_sequence) {
            directRoutes.push({
              route: shape,
              startStop,
              destStop,
              tripId
            })
            seenRoutes.add(routeId)
            foundForThisShape = true
            break
          }
        }
        if (foundForThisShape) break
      }
    }
  }

  return directRoutes
}
