import {INCOMING_SUFFIX, type RouteDirection, type ShapeInfo, type StopTime} from '@/types/tranzy.ts'

export const WALKING_SPEED_M_PER_MIN = 80

export const getRouteIdFromTripId = (tripId: string): number | null => {
  const [routeIdPart] = String(tripId).split('_')
  const routeId = Number(routeIdPart)
  return Number.isFinite(routeId) ? routeId : null
}

export const getDirectionFromTripId = (tripId: string): RouteDirection =>
  tripId.endsWith(INCOMING_SUFFIX) ? '1' : '0'

export const getTripIdForRouteAtStop =(outgoingTripIds: string[], incomingTripIds: string[], routeId: number | string): string | undefined => {
  const wantedRouteId = Number(routeId)
  if (!Number.isFinite(wantedRouteId)) return undefined
  return [...(outgoingTripIds || []), ...(incomingTripIds || [])]
    .find((id) => getRouteIdFromTripId(id) === wantedRouteId)
}

// Minutes from the trip's departure until it reaches `stopId`. offset_arrival_time
// is the ride from the previous stop *into* this one, so the target's own delta is
// part of the answer - dropping it reported every stop at the previous stop's time.
export const getTimeOffsetToStop = (stopTimes: StopTime[], tripId: string, stopId: number): number => {
  const ordered = stopTimes
    .filter((st) => st.trip_id === tripId)
    .sort((a, b) => a.stop_sequence - b.stop_sequence)
  let timeOffsetSec = 0
  for (const st of ordered) {
    timeOffsetSec += st.offset_arrival_time
    if (st.stop_id === stopId) break
  }
  return timeOffsetSec
}

export const getShapeStopTimes = (shapeInfo: ShapeInfo | null | undefined): StopTime[] => {
  return shapeInfo?.stop_times ?? shapeInfo?.stop_time ?? []
}
