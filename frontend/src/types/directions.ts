export type DirectionsSummary = {
  distance: number // meters
  duration: number // seconds
}

export type DirectionsRoute = {
  summary: DirectionsSummary
  geometry: string // Google-encoded polyline
}

export type DirectionsResponse = {
  routes: DirectionsRoute[]
}
