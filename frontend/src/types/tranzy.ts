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
