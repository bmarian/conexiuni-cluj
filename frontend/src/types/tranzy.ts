import type {Timetable} from "@/types/ctp.ts";

export type UserLocation = {
  latitude: number
  longitude: number
}

export type StopTime = {
  trip_id: string
  stop_id: number
  offset_arrival_time: number
  stop_sequence: number
  stop_headsign: string
  route_short_name: string
  stop_lat: number
  stop_lon: number
}

export type RouteType =
  | 0  // Tram
  | 1  // Subway
  | 2  // Rail
  | 3  // Bus
  | 4  // Ferry
  | 5  // CableTram
  | 6  // AerialLift
  | 7  // Funicular
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

export const OUTGOING_SUFFIX = "_0"
export const INCOMING_SUFFIX = "_1"

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
}

export type Stop = {
  stop_id: number
  stop_name: string
  stop_desc: string
  stop_lat: number
  stop_lon: number
  location_type: LocationType
  stop_code: string
}

export type Shape = {
  shape_id: string
  shape_pt_lat: number
  shape_pt_lon: number
  shape_pt_sequence: number
  shape_dist_traveled: number
}

export type Vehicle = {
  id: number
  label: string
  latitude: number
  longitude: number
  timestamp: string
  vehicle_type: number
  bike_accessible: string
  wheelchair_accessible: string
  speed: number
  route_id: number
  trip_id: string
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
}

export type DirectionsResponse = {
  routes: {
    geometry: string
  }[]
}
