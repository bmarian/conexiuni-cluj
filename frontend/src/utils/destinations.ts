import {OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import type {NetworkIndex, NetworkRoute} from '@/utils/network.ts'
import {reverseRouteLongName, routeDestination} from '@/utils/time.ts'

export type Destination = {name: string | null, routes: NetworkRoute[]}

export function stationDestinations(index: NetworkIndex, stopId: number): Destination[] {
  const platforms = new Set(index.platforms(stopId).map((s) => s.stop_id))
  const out = new Map<string | null, NetworkRoute[]>()
  const leaving = new Set<NetworkRoute>()
  const ending: NetworkRoute[] = []
  for (const route of index.look(stopId)?.routes ?? []) {
    for (const trip of route.trips) {
      const at = trip.stop_ids.findIndex((id) => platforms.has(id))
      if (at < 0) continue
      if (at === trip.stop_ids.length - 1) {
        if (!ending.includes(route)) ending.push(route)
        continue
      }
      leaving.add(route)
      const longName = route.timetable?.route_long_name || route.route_long_name
      const name = routeDestination(trip.trip_id.endsWith(OUTGOING_SUFFIX) ? longName : reverseRouteLongName(longName))
      const routes = out.get(name)
      if (!routes) out.set(name, [route])
      else if (!routes.includes(route)) routes.push(route)
    }
  }
  const ends = ending.filter((route) => !leaving.has(route))
  if (ends.length) out.set(null, ends)
  return [...out].map(([name, routes]) => ({name, routes}))
}
