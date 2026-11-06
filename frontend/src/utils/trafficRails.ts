import type {Map as MapLibreMap} from 'maplibre-gl'
import {CLEAR, growLine, paintLine, stopSweep} from '@/utils/lineSweep.ts'
import {type Traffic, trafficPaint} from '@/utils/traffic.ts'

const GAP = 1.5
const RAILS = [
  {band: 2, width: 2},
  {band: 3, width: 3},
]

const railId = (id: string, band: number) => `${id}-rails-${band}`

export function syncRails(m: MapLibreMap, id: string, line: {coords: [number, number][]; width: number}, traffic: Traffic | undefined) {
  for (const {band, width} of RAILS) {
    const rid = railId(id, band)
    const stretches = traffic?.stretches.filter((s) => s.band === band) ?? []
    if (!stretches.length) {
      dropRail(m, rid)
      continue
    }
    if (!m.getLayer(rid)) {
      const layout = {'line-cap': 'round', 'line-join': 'round'} as const
      m.addLayer({id: rid, type: 'line', source: id, layout, paint: {'line-color': CLEAR, 'line-width': width}}, id)
    }
    m.setPaintProperty(rid, 'line-gap-width', line.width + 2 * GAP)
    paintLine(m, rid, trafficPaint(stretches, line.coords, CLEAR))
  }
}

function dropRail(m: MapLibreMap, rid: string) {
  stopSweep(m, rid)
  paintLine(m, rid)
  if (m.getLayer(rid)) m.removeLayer(rid)
}

export function dropRails(m: MapLibreMap, id: string) {
  for (const {band} of RAILS) dropRail(m, railId(id, band))
}

export const growRails = (m: MapLibreMap, id: string) =>
  RAILS.map(({band}) => railId(id, band))
    .filter((rid) => m.getLayer(rid))
    .map((rid) => growLine(m, rid, CLEAR))
