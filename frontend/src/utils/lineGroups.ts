import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type RouteDirection} from '@/types/tranzy.ts'
import type {NetworkIndex, NetworkRoute} from '@/utils/network.ts'
import {GROUP_LINES_CAP} from '@/utils/sync/records.ts'

export type GroupLine = {routeId: number; direction: RouteDirection}

const INTORS = '.intors'

export const tripOf = (line: GroupLine) => `${line.routeId}${line.direction === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`

export const groupKey = (lines: GroupLine[]) => lines.map((l) => `${l.routeId}/${l.direction}`).join(',')

export function readKey(index: NetworkIndex, key: string): GroupLine[] {
  return key.split(',').flatMap((part): GroupLine[] => {
    const [id, direction] = part.split('/')
    const routeId = Number(id)
    return index.routeById.has(routeId) && (direction === '0' || direction === '1') ? [{routeId, direction}] : []
  })
}

export function writeLines(index: NetworkIndex, lines: GroupLine[]): string {
  return lines
    .map((l) => {
      const slug = index.routeById.get(l.routeId)?.slug || `~${l.routeId}`
      return l.direction === '1' ? `${slug}${INTORS}` : slug
    })
    .join(',')
}

export function readLines(index: NetworkIndex, raw: string): GroupLine[] {
  const lines: GroupLine[] = []
  for (const part of raw.split(',')) {
    if (lines.length === GROUP_LINES_CAP) break
    const direction: RouteDirection = part.endsWith(INTORS) ? '1' : '0'
    const slug = direction === '1' ? part.slice(0, -INTORS.length) : part
    const routeId = slug.startsWith('~') ? Number(slug.slice(1)) : index.routeBySlug.get(slug)
    if (routeId === undefined || !index.routeById.has(routeId) || lines.some((l) => l.routeId === routeId)) continue
    lines.push({routeId, direction})
  }
  return lines
}

export const groupRoutes = (index: NetworkIndex, lines: GroupLine[]): NetworkRoute[] =>
  lines.flatMap((l) => {
    const route = index.routeById.get(l.routeId)
    return route ? [route] : []
  })

export const groupLabel = (routes: NetworkRoute[]) => routes.map((r) => r.route_short_name).join(', ')

export function stopsOf(index: NetworkIndex, line: GroupLine): number[] {
  return index.routeById.get(line.routeId)?.trips.find((t) => t.trip_id === tripOf(line))?.stop_ids ?? []
}

// Stations where two of the lines run together on one side and apart on the other.
export function divergingStops(index: NetworkIndex, lines: GroupLine[]): number[] {
  const visits = new Map<number, {stop: number; prev?: number; next?: number}[]>()
  for (const line of lines) {
    const stops = stopsOf(index, line)
    stops.forEach((stop, i) => {
      const station = index.stationId(stop)
      const visit = {stop, prev: i > 0 ? index.stationId(stops[i - 1]!) : undefined, next: i < stops.length - 1 ? index.stationId(stops[i + 1]!) : undefined}
      visits.set(station, [...(visits.get(station) ?? []), visit])
    })
  }
  return [...visits.values()].flatMap((list) =>
    list.some((a, i) => list.slice(i + 1).some((b) => (a.prev === b.prev) !== (a.next === b.next))) ? [list[0]!.stop] : [],
  )
}
