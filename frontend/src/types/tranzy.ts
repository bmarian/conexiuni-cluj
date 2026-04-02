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
