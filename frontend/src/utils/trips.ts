import type {ShapeInfo, StopTime} from '@/types/tranzy.ts'

export const getRouteIdFromTripId = (tripId: string): number | null => {
  const [routeIdPart] = String(tripId).split('_')
  const routeId = Number(routeIdPart)
  return Number.isFinite(routeId) ? routeId : null
}

export const getShapeStopTimes = (shapeInfo: ShapeInfo | null | undefined): StopTime[] => {
  return shapeInfo?.stop_times ?? shapeInfo?.stop_time ?? []
}
