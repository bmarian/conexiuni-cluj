import type {ShapeInfo, StopInfo, StopTime} from '@/types/tranzy.ts'

export type DirectRoute = {
  route: ShapeInfo
  startStop: StopInfo
  destStop: StopInfo
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
