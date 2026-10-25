import type {Timetable} from '@/types/ctp.ts'

export type UserLocation = {
  latitude: number
  longitude: number
}

export type StopTime = {
  trip_id: string
  stop_id: number
  offset_arrival_time: number
  offset_confidence?: number
  stop_sequence: number
  stop_headsign: string
  route_short_name: string
  stop_lat: number
  stop_lon: number
}

export type HourlyStopOffsets = {
  stop_ids: number[]
  hourly_offset_seconds: Record<string, number[]>
}

export type RouteType =
  | 0 // Tram
  | 1 // Subway
  | 2 // Rail
  | 3 // Bus
  | 4 // Ferry
  | 5 // CableTram
  | 6 // AerialLift
  | 7 // Funicular
  | 11 // Trolleybus
  | 12 // Monorail

export type Route = {
  route_id: number
  agency_id: number
  route_short_name: string
  route_long_name: string
  route_type: RouteType
  route_desc: string
  route_color: string
}

export const OUTGOING_SUFFIX = '_0'
export const INCOMING_SUFFIX = '_1'

export type RouteDirection = '0' | '1'

export type LocationType =
  | 0 // StopOrPlatform
  | 1 // Station
  | 2 // EntranceExit
  | 3 // GenericNode
  | 4 // BoardingArea

export type ShapeInfo = {
  route_short_name: string
  route_long_name: string
  route_id: number
  route_type: RouteType
  route_color: string
  stop_times?: StopTime[]
  stop_time?: StopTime[]
  timetable: Timetable
  // A timetable run this many minutes late is still coming (learned on the server).
  late_grace_min?: number
}

export type StopInfo = {
  stop_id: number
  stop_name: string
  stop_desc: string
  stop_lat: number
  stop_lon: number
  location_type: LocationType
  stop_code: string
  outgoing_trip_ids: string[]
  incoming_trip_ids: string[]
  shapes_info: ShapeInfo[]
  late_grace_min?: number
}

export type Stop = {
  stop_id: number
  stop_name: string
  stop_desc: string
  stop_lat: number
  stop_lon: number
  location_type: LocationType
  stop_code: string
  station_id?: number
}

export type Shape = {
  shape_id: string
  shape_pt_lat: number
  shape_pt_lon: number
  shape_pt_sequence: number
  shape_dist_traveled: number
}

// A vehicle as the live feed reports it: snapped and headed by the server.
export type LiveVehicle = {
  id: number
  trip_id: string
  route_id: number
  latitude: number
  longitude: number
  heading: number
  // Reported km/h, before any smoothing.
  speed?: number
  timestamp: string
  label?: string
  vehicle_type?: number
  bike_accessible?: string
  wheelchair_accessible?: string
}

export type Vehicle = LiveVehicle

export type LiveEntry = {
  // Predicted arrival, unix seconds.
  at: number
  live: boolean
  soon?: boolean
  vehicle_id?: number
}

// One stop of one trip as the server predicts it; `minutes` count from the row's as_of.
export type LiveRow = {
  trip_id: string
  stop_id: number
  entries: LiveEntry[]
}

export type TimeEntry = {
  minutes: number
  is_live: boolean
}

export type VehiclesInStop = {
  minutes_left: number
  next_times: TimeEntry[]
  route_short_name: string
  route_type: RouteType
  route_color: string
  trip_id: string
  route_id: number
  route_long_name: string
  static_time_approximation: boolean
  vehicles?: Vehicle[]
  stop_id?: number
}
