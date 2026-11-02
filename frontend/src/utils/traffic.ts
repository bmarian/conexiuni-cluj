import type {Feature, FeatureCollection, LineString} from 'geojson'
import type {ExpressionSpecification} from 'maplibre-gl'
import {decodePolyline} from '@/utils/network.ts'

export type TrafficStretch = {path: string; band: number}
export type TrafficNow = {lines: Record<string, TrafficStretch[]>}

// Fast to slow, the traffic-light scale map readers know.
export const TRAFFIC_COLORS = ['#34a853', '#fbbc04', '#ea4335', '#a50e0e'] as const

// The stretches of the drawn lines, each with its band.
export function trafficFeatures(now: TrafficNow, tripIds: Iterable<string>): FeatureCollection<LineString> {
  const features: Feature<LineString>[] = []
  for (const id of new Set(tripIds)) {
    for (const stretch of now.lines[id] ?? []) {
      features.push({
        type: 'Feature',
        properties: {band: stretch.band},
        geometry: {type: 'LineString', coordinates: decodePolyline(stretch.path).map(([lat, lon]) => [lon, lat])},
      })
    }
  }
  return {type: 'FeatureCollection', features}
}

// One colour per band of the city's own slowdowns.
export const trafficColor: ExpressionSpecification = ['step', ['get', 'band'], TRAFFIC_COLORS[0], 1, TRAFFIC_COLORS[1], 2, TRAFFIC_COLORS[2], 3, TRAFFIC_COLORS[3]]
