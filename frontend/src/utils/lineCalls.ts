import type {RouteDirection} from '@/types/tranzy.ts'
import type {NetworkIndex, NetworkRoute} from '@/utils/network.ts'
import {lineStops} from '@/utils/lineGroups.ts'
import {byModeThenNumber} from '@/utils/modes.ts'

export type Call = {route: NetworkRoute; direction: RouteDirection; stopId: number}

const DIRECTIONS: RouteDirection[] = ['0', '1']
const VIA_SHOWN = 3

export function callsByStation(index: NetworkIndex, routes: NetworkRoute[]): Map<number, Call[]> {
  const out = new Map<number, Call[]>()
  for (const route of [...routes].sort(byModeThenNumber)) {
    for (const direction of DIRECTIONS) {
      const seen = new Set<number>()
      for (const stopId of lineStops(index, {routeId: route.route_id, direction}).slice(0, -1)) {
        const station = index.stationId(stopId)
        if (seen.has(station)) continue
        seen.add(station)
        const call = {route, direction, stopId}
        const list = out.get(station)
        if (list) list.push(call)
        else out.set(station, [call])
      }
    }
  }
  return out
}

// The stations most lines call at, in the line's order.
export function viaStops(index: NetworkIndex, route: NetworkRoute, calls: Map<number, Call[]>): string[] {
  const stops = DIRECTIONS.map((direction) => lineStops(index, {routeId: route.route_id, direction})).find((list) => list.length) ?? []
  const seen = new Set<number>()
  const between = stops.slice(1, -1).flatMap((stopId, at) => {
    const station = index.stationId(stopId)
    if (seen.has(station)) return []
    seen.add(station)
    return [{stopId, at, lines: new Set(calls.get(station)?.map((c) => c.route.route_id)).size}]
  })
  return between
    .sort((a, b) => b.lines - a.lines || a.at - b.at)
    .slice(0, VIA_SHOWN)
    .sort((a, b) => a.at - b.at)
    .map((s) => index.stopName(s.stopId))
}
