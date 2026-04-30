import type {ShapeInfo, StopTime} from '@/types/tranzy.ts'

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
