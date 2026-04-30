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
      const stopTimes = getShapeStopTimes(shape)

      // Group stopTimes by trip_id once per shape
      const tripGroups = new Map<string, StopTime[]>()
      for (const st of stopTimes) {
        if (!tripGroups.has(st.trip_id)) {
          tripGroups.set(st.trip_id, [])
        }
        tripGroups.get(st.trip_id)!.push(st)
      }

      for (const [tripId, tripStopTimes] of tripGroups) {
        const startStopTime = tripStopTimes.find(st => st.stop_id === startStop.stop_id)
        if (!startStopTime) continue

        for (const destStop of destStops) {
          const destStopTime = tripStopTimes.find(st => st.stop_id === destStop.stop_id)

          if (destStopTime && startStopTime.stop_sequence < destStopTime.stop_sequence) {
            const key = `${shape.route_id}-${startStop.stop_id}-${destStop.stop_id}`
            if (!seenRoutes.has(key)) {
              directRoutes.push({
                route: shape,
                startStop,
                destStop,
                tripId
              })
              seenRoutes.add(key)
            }
          }
        }
      }
    }
  }

  return directRoutes
}
