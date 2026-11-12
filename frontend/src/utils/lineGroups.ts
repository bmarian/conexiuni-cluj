import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type RouteDirection, type UserLocation} from '@/types/tranzy.ts'
import type {NetworkIndex, NetworkRoute} from '@/utils/network.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {GROUP_LINES_CAP, validId} from '@/utils/sync/records.ts'

// No from: the stop nearest you. No to: the end of the line.
export type GroupLine = {routeId: number; direction: RouteDirection; from?: number; to?: number}

export type GroupLegs = {lines: GroupLine[]; back?: GroupLine[]}

export type Leg = 'dus' | 'intors'

export type StretchEnd = 'from' | 'to'

export type StopRole = 'board' | 'near' | 'leave' | 'pass'
export type StopRow = {stopId: number; role: StopRole; painted: {top: boolean; bottom: boolean}; dimmed: boolean}

const INTORS = '.intors'
const STOPS = ':'

export const tripOf = (line: GroupLine) => `${line.routeId}${line.direction === '0' ? OUTGOING_SUFFIX : INCOMING_SUFFIX}`

export const opposite = (direction: RouteDirection): RouteDirection => (direction === '0' ? '1' : '0')

const hasStops = (line: GroupLine) => line.from !== undefined || line.to !== undefined

export const lineKey = (line: GroupLine) => `${line.routeId}/${line.direction}${hasStops(line) ? `/${line.from ?? ''}/${line.to ?? ''}` : ''}`

export const groupKey = (lines: GroupLine[]) => lines.map(lineKey).join(',')

export const legsKey = (legs: GroupLegs) => (legs.back?.length ? `${groupKey(legs.lines)}|${groupKey(legs.back)}` : groupKey(legs.lines))

export const legsOf = (lines: GroupLine[], back: GroupLine[] | undefined): GroupLegs => (back?.length ? {lines, back} : {lines})

export const legLines = (legs: GroupLegs, leg: Leg) => (leg === 'intors' ? (legs.back ?? []) : legs.lines)

export const withStops = (line: GroupLine, from: number | undefined, to: number | undefined): GroupLine => ({
  routeId: line.routeId,
  direction: line.direction,
  ...(from === undefined ? {} : {from}),
  ...(to === undefined ? {} : {to}),
})

export const lineStops = (index: NetworkIndex, line: GroupLine): number[] => index.tripStopIds(tripOf(line))

// from is -1 and to the last stop where none is chosen, or the chosen one is no longer on the line in that order.
export function stretchOf(index: NetworkIndex, line: GroupLine): {stops: number[]; from: number; to: number} {
  const stops = lineStops(index, line)
  const last = stops.length - 1
  const boards = line.from === undefined ? -1 : stops.indexOf(line.from)
  const from = boards >= last ? -1 : boards
  const leaves = line.to === undefined ? -1 : stops.indexOf(line.to, Math.max(from, 0) + 1)
  return {stops, from, to: leaves < 0 ? last : leaves}
}

function chosenStops(index: NetworkIndex, line: GroupLine): {from?: number; to?: number} {
  const {stops, from, to} = stretchOf(index, line)
  return {...(from < 0 ? {} : {from: stops[from]}), ...(to >= stops.length - 1 ? {} : {to: stops[to]})}
}

function settled(index: NetworkIndex, line: GroupLine): GroupLine {
  const {from, to} = chosenStops(index, line)
  return withStops(line, from, to)
}

export function writeLines(index: NetworkIndex, lines: GroupLine[]): string {
  return lines
    .map((l) => {
      const slug = index.routeById.get(l.routeId)?.slug || `~${l.routeId}`
      const stops = hasStops(l) ? `${STOPS}${l.from ?? ''}${l.to === undefined ? '' : `${STOPS}${l.to}`}` : ''
      return `${slug}${l.direction === '1' ? INTORS : ''}${stops}`
    })
    .join(',')
}

const stopNumber = (text: string | undefined) => (text && validId(Number(text)) ? Number(text) : undefined)

export function readLines(index: NetworkIndex, raw: string): GroupLine[] {
  const lines: GroupLine[] = []
  for (const part of raw.split(',')) {
    if (lines.length === GROUP_LINES_CAP) break
    const [name = '', from, to] = part.split(STOPS)
    const direction: RouteDirection = name.endsWith(INTORS) ? '1' : '0'
    const slug = direction === '1' ? name.slice(0, -INTORS.length) : name
    const routeId = slug.startsWith('~') ? Number(slug.slice(1)) : index.routeBySlug.get(slug)
    if (routeId === undefined || !index.routeById.has(routeId) || lines.some((l) => l.routeId === routeId)) continue
    lines.push(settled(index, withStops({routeId, direction}, stopNumber(from), stopNumber(to))))
  }
  return lines
}

export const groupRoutes = (index: NetworkIndex, lines: GroupLine[]): NetworkRoute[] =>
  lines.flatMap((l) => {
    const route = index.routeById.get(l.routeId)
    return route ? [route] : []
  })

export const groupLabel = (routes: NetworkRoute[]) => routes.map((r) => r.route_short_name).join(', ')

// The chosen stop; else the line's stop nearest you before the get-off stop, its first without a location.
export function boardingStop(index: NetworkIndex, line: GroupLine, loc: UserLocation | null): number | undefined {
  const {stops, from, to} = stretchOf(index, line)
  if (from >= 0) return stops[from]
  const boardable = stops.slice(0, to)
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

function boardingDistance(index: NetworkIndex, lines: GroupLine[], loc: UserLocation): number {
  return Math.min(
    ...lines.map((line) => {
      const stop = index.stopById.get(boardingStop(index, line, loc) ?? -1)
      return stop ? haversineMeters(loc.latitude, loc.longitude, stop.stop_lat, stop.stop_lon) : Infinity
    }),
  )
}

// The way there on a tie.
export function nearerLeg(index: NetworkIndex, legs: GroupLegs, loc: UserLocation): Leg {
  if (!legs.back?.length) return 'dus'
  return boardingDistance(index, legs.back, loc) < boardingDistance(index, legs.lines, loc) ? 'intors' : 'dus'
}

export function stretchNames(index: NetworkIndex, line: GroupLine): {from?: string; to?: string} {
  const {from, to} = chosenStops(index, line)
  return {...(from === undefined ? {} : {from: index.stopName(from)}), ...(to === undefined ? {} : {to: index.stopName(to)})}
}

export function flipLine(index: NetworkIndex, line: GroupLine): GroupLine {
  const other: GroupLine = {routeId: line.routeId, direction: opposite(line.direction)}
  const {from, to} = chosenStops(index, line)
  const back = lineStops(index, other)
  const across = (stopId: number | undefined) => {
    const station = stopId === undefined ? undefined : index.stationId(stopId)
    return back.find((id) => index.stationId(id) === station)
  }
  return settled(index, withStops(other, across(to), across(from)))
}

export function pickStop(index: NetworkIndex, line: GroupLine, end: StretchEnd, stopId: number): {line: GroupLine; next: StretchEnd} {
  const {stops, from, to} = stretchOf(index, line)
  const {from: boards, to: leaves} = chosenStops(index, line)
  const last = stops.length - 1
  const at = stops.indexOf(stopId)
  if (at < 0) return {line, next: end}
  if (end === 'from') {
    if (at >= last) return {line, next: end}
    if (at === from) return {line: withStops(line, undefined, leaves), next: end}
    return {line: withStops(line, stopId, to > at ? leaves : undefined), next: 'to'}
  }
  if (at === to) return {line: withStops(line, boards, undefined), next: end}
  if (from >= 0 && at <= from) return {line: withStops(line, stopId, undefined), next: end}
  if (at === 0) return {line, next: end}
  return {line: withStops(line, boards, at === last ? undefined : stopId), next: end}
}

export function stopRows(index: NetworkIndex, line: GroupLine, nearest: number | undefined): StopRow[] {
  const {stops, from, to} = stretchOf(index, line)
  const near = nearest === undefined ? -1 : stops.indexOf(nearest)
  const start = from >= 0 ? from : Math.max(near, 0)
  const leaves = to < stops.length - 1
  return stops.map((stopId, at) => ({
    stopId,
    role: at === from ? 'board' : at === near ? 'near' : at === to && leaves ? 'leave' : 'pass',
    painted: {top: at > start && at <= to, bottom: at >= start && at < to},
    dimmed: at < start || at > to,
  }))
}
