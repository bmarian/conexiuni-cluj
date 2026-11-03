import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type RouteDirection, type UserLocation} from '@/types/tranzy.ts'
import type {NetworkIndex} from '@/utils/network.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {GROUP_LINES_CAP} from '@/utils/sync/records.ts'

export type GroupLine = {routeId: number; direction: RouteDirection}

export type LegLine = GroupLine & {stopId: number}

export type Leg = {lines: LegLine[]; alight?: LegLine[]}

export type Departure = {routeId: number; at: number; live: boolean; vehicleId?: number}

export type LegTimes = {
  shown: Departure[]
  missed?: Departure
  caught?: Departure
  arrival?: number
}

export type ChainLeg = {lines: {name: string; color: string}[]; change: boolean}

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

export function stopsOf(index: NetworkIndex, line: GroupLine): number[] {
  return index.routeById.get(line.routeId)?.trips.find((t) => t.trip_id === tripOf(line))?.stop_ids ?? []
}

export function nearestBoarding(index: NetworkIndex, stops: number[], loc: UserLocation | null): number {
  const boardable = stops.slice(0, -1)
  let best = boardable[0]!
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

// Changes are found from each line's first stop, so where you are never hides one.
export function legsNear(index: NetworkIndex, lines: GroupLine[], loc: UserLocation | null): Leg[] {
  const plain = planLegs(index, lines, (stops) => stops[0]!)
  const leftAt = new Map<number, number>()
  plain.forEach((_, i) => {
    for (const a of plain[i + 1]?.alight ?? []) leftAt.set(a.routeId, index.stationId(a.stopId))
  })
  return planLegs(index, lines, (stops, line) => {
    const station = leftAt.get(line.routeId)
    const at = station === undefined ? -1 : stops.findIndex((id) => index.stationId(id) === station)
    return nearestBoarding(index, at > 0 ? stops.slice(0, at + 1) : stops, loc)
  })
}

// Where the line leaves the leg's way after boarding, else where it first meets it.
function change(index: NetworkIndex, leg: Leg, stops: number[]): {board: number; alight: LegLine[]} | null {
  let best: {rode: number; station: number; board: number} | null = null
  for (const prev of leg.lines) {
    const prevStops = stopsOf(index, prev)
    const from = prevStops.indexOf(prev.stopId)
    const position = new Map<number, number>()
    prevStops.slice(from + 1).forEach((id, i) => {
      const station = index.stationId(id)
      if (!position.has(station)) position.set(station, from + 1 + i)
    })
    let found: {rode: number; station: number; board: number} | null = null
    for (let i = from + 1; i < prevStops.length; i++) {
      const station = index.stationId(prevStops[i]!)
      const at = stops.findIndex((id, k) => k < stops.length - 1 && index.stationId(id) === station)
      if (at < 0) continue
      const meet = {rode: i - from, station, board: stops[at]!}
      found ??= meet
      const next = position.get(index.stationId(stops[at + 1]!))
      if (next !== undefined && next > i) continue
      found = meet
      break
    }
    if (found && (!best || found.rode < best.rode)) best = found
  }
  if (!best) return null
  const alight = leg.lines.flatMap((prev) => {
    const prevStops = stopsOf(index, prev)
    const stopId = prevStops.slice(prevStops.indexOf(prev.stopId) + 1).find((id) => index.stationId(id) === best.station)
    return stopId === undefined ? [] : [{routeId: prev.routeId, direction: prev.direction, stopId}]
  })
  return {board: best.board, alight}
}

// A line through the boarding station is another way to ride, not a change.
export function meetingLines(index: NetworkIndex, line: LegLine): (LegLine & {rode: number})[] {
  const stops = stopsOf(index, line)
  const boarding = index.stationId(line.stopId)
  const position = new Map<number, number>()
  stops.slice(stops.indexOf(line.stopId) + 1).forEach((id, i) => {
    const station = index.stationId(id)
    if (!position.has(station)) position.set(station, i + 1)
  })
  const found = new Map<number, LegLine & {rode: number}>()
  for (const route of index.bundle.routes) {
    if (route.route_id === line.routeId || route.trips.some((t) => t.stop_ids.some((id) => index.stationId(id) === boarding))) continue
    for (const trip of route.trips) {
      trip.stop_ids.slice(0, -1).forEach((stopId, k) => {
        const rode = position.get(index.stationId(stopId))
        const next = position.get(index.stationId(trip.stop_ids[k + 1]!))
        const known = found.get(route.route_id)
        if (rode === undefined || (next !== undefined && next > rode) || (known && known.rode <= rode)) return
        found.set(route.route_id, {routeId: route.route_id, direction: trip.trip_id.endsWith(INCOMING_SUFFIX) ? '1' : '0', stopId, rode})
      })
    }
  }
  return [...found.values()].sort((a, b) => a.rode - b.rode)
}

export function planLegs(index: NetworkIndex, lines: GroupLine[], startStop: (stops: number[], line: GroupLine) => number): Leg[] {
  const legs: Leg[] = []
  for (const line of lines) {
    const stops = stopsOf(index, line)
    if (stops.length < 2) continue
    const leg = legs.at(-1)
    if (leg) {
      const first = leg.lines[0]!
      const firstStops = stopsOf(index, first)
      const boardAt = firstStops.indexOf(first.stopId)
      const here = stops.findIndex((id, k) => k < stops.length - 1 && index.stationId(id) === index.stationId(first.stopId))
      if (here >= 0) {
        const sameWay = boardAt >= 0 && boardAt < firstStops.length - 1 && index.stationId(stops[here + 1]!) === index.stationId(firstStops[boardAt + 1]!)
        if (sameWay) leg.lines.push({...line, stopId: stops[here]!})
        else legs.push({lines: [{...line, stopId: stops[here]!}]})
        continue
      }
      const changed = change(index, leg, stops)
      if (changed) {
        legs.push({lines: [{...line, stopId: changed.board}], alight: changed.alight})
        continue
      }
    }
    legs.push({lines: [{...line, stopId: startStop(stops, line)}]})
  }
  return legs
}

export function timeLegs(
  legs: Leg[],
  departures: (line: LegLine) => Departure[],
  arrivals: (line: LegLine) => Departure[],
  walkMs: (from: number, to: number) => number,
  shown: number,
): LegTimes[] {
  const out: LegTimes[] = []
  legs.forEach((leg, i) => {
    const all = leg.lines.flatMap(departures).sort((a, b) => a.at - b.at)
    const before = out[i - 1]?.caught
    const alight = before && leg.alight?.find((l) => l.routeId === before.routeId)
    if (!before || !alight) {
      out.push({shown: all.slice(0, shown), caught: all[0]})
      return
    }
    const coming = arrivals(alight).filter((a) => a.at > before.at)
    const reached = (before.vehicleId !== undefined && coming.find((a) => a.vehicleId === before.vehicleId)) || coming[0]
    if (!reached) {
      out.push({shown: all.slice(0, shown)})
      return
    }
    const ready = (line: Departure) => reached.at + walkMs(alight.stopId, leg.lines.find((l) => l.routeId === line.routeId)!.stopId)
    const catchable = all.filter((d) => d.at >= ready(d))
    out.push({
      shown: catchable.slice(0, shown),
      missed: all.filter((d) => d.at < ready(d)).at(-1),
      caught: catchable[0],
      arrival: reached.at,
    })
  })
  return out
}

export function chainOf(index: NetworkIndex, legs: Leg[], color: (name: string, color: string) => string): ChainLeg[] {
  return legs.map((leg) => ({
    change: !!leg.alight,
    lines: leg.lines.flatMap((l) => {
      const route = index.routeById.get(l.routeId)
      return route ? [{name: route.route_short_name, color: color(route.route_short_name, route.route_color)}] : []
    }),
  }))
}

export const chainText = (chain: ChainLeg[]) =>
  chain.map((leg, i) => `${i === 0 ? '' : leg.change ? ' → ' : ' + '}${leg.lines.map((l) => l.name).join(' / ')}`).join('')
