import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type RouteDirection, type UserLocation} from '@/types/tranzy.ts'
import type {NetworkIndex, NetworkRoute} from '@/utils/network.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {GROUP_LINES_CAP} from '@/utils/sync/records.ts'

export type GroupLine = {routeId: number; direction: RouteDirection}

const INTORS = '.intors'

export const tripOf = (line: GroupLine) => `${line.routeId}${line.direction === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`

export const groupKey = (lines: GroupLine[]) => lines.map((l) => `${l.routeId}/${l.direction}`).join(',')

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

function stopsOf(index: NetworkIndex, line: GroupLine): number[] {
  return index.routeById.get(line.routeId)?.trips.find((t) => t.trip_id === tripOf(line))?.stop_ids ?? []
}

// The line's stop nearest you, never its last; its first without a location.
export function boardingStop(index: NetworkIndex, line: GroupLine, loc: UserLocation | null): number | undefined {
  const boardable = stopsOf(index, line).slice(0, -1)
  let best = boardable[0]
  let bestDist = Infinity
  for (const id of loc ? boardable : []) {
    const stop = index.stopById.get(id)
    if (!stop) continue
    const d = haversineMeters(loc!.latitude, loc!.longitude, stop.stop_lat, stop.stop_lon)
    if (d < bestDist) {
      bestDist = d
      best = id
    }
  }
  return best
}
