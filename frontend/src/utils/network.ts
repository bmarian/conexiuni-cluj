import {shallowRef} from 'vue'
import type {HourlyStopOffsets, Route, ShapeInfo, Stop, StopInfo, StopTime} from '@/types/tranzy.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import type {Timetable} from '@/types/ctp.ts'
import {hasTimetableEntries} from '@/utils/time.ts'

export type NetworkTrip = {
  trip_id: string
  stop_ids: number[]
  stop_sequences: number[]
  shape_hash?: string
}

export type NetworkRoute = Route & {
  timetable: Timetable | null
  trips: NetworkTrip[]
  offsets: Record<string, number[][]>
}

export type NetworkBundle = {
  version: string
  static_version: string
  generated_at: string
  routes: NetworkRoute[]
  stops: Stop[]
}

export type NetworkVersion = Pick<NetworkBundle, 'version' | 'static_version' | 'generated_at'>

export type DayType = 'weekday' | 'saturday' | 'sunday'

export class NetworkIndex {
  readonly stopById = new Map<number, Stop>()
  readonly routeById = new Map<number, NetworkRoute>()
  readonly routeByShortName = new Map<string, NetworkRoute>()
  readonly routes: Route[]

  constructor(readonly bundle: NetworkBundle) {
    for (const s of bundle.stops) this.stopById.set(s.stop_id, s)
    for (const r of bundle.routes) {
      this.routeById.set(r.route_id, r)
      this.routeByShortName.set(r.route_short_name, r)
    }
    this.routes = bundle.routes.map(({route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color}) =>
      ({route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color}))
  }

  shapeHash(tripId: string): string | undefined {
    const route = this.routeById.get(Number(tripId.split('_')[0]))
    return route?.trips.find((t) => t.trip_id === tripId)?.shape_hash
  }
}

const CLUJ_TIME = new Intl.DateTimeFormat('en-GB', {timeZone: 'Europe/Bucharest', weekday: 'short', hour: 'numeric', hourCycle: 'h23'})

export function clujClock(at: Date = new Date()): {dayType: DayType, hour: number} {
  const parts = CLUJ_TIME.formatToParts(at)
  const weekday = parts.find((p) => p.type === 'weekday')?.value
  const hour = Number(parts.find((p) => p.type === 'hour')?.value ?? 0) % 24
  return {dayType: weekday === 'Sat' ? 'saturday' : weekday === 'Sun' ? 'sunday' : 'weekday', hour}
}

export function routeStopTimes(index: NetworkIndex, route: NetworkRoute, dayType: DayType, hour: number): StopTime[] {
  const offsets = route.offsets[dayType]?.[hour] ?? []
  const out: StopTime[] = []
  let i = 0
  for (const trip of route.trips) {
    trip.stop_ids.forEach((stopId, k) => {
      const stop = index.stopById.get(stopId)
      out.push({
        trip_id: trip.trip_id,
        stop_id: stopId,
        offset_arrival_time: offsets[i] ?? 0,
        stop_sequence: trip.stop_sequences[k] ?? 0,
        stop_headsign: stop?.stop_name ?? '',
        route_short_name: route.route_short_name,
        stop_lat: stop?.stop_lat ?? 0,
        stop_lon: stop?.stop_lon ?? 0,
      })
      i++
    })
  }
  return out
}

export function buildStopInfo(index: NetworkIndex, stopId: number, at: Date = new Date()): StopInfo | null {
  const stop = index.stopById.get(stopId)
  if (!stop) return null
  const {dayType, hour} = clujClock(at)
  const trips = new Set<string>()
  const shapes: ShapeInfo[] = []
  for (const route of [...index.bundle.routes].sort((a, b) => a.route_id - b.route_id)) {
    const here = route.trips.filter((t) => t.stop_ids.includes(stopId)).map((t) => t.trip_id)
    if (!here.length) continue
    here.forEach((id) => trips.add(id))
    if (!route.timetable || !hasTimetableEntries(route.timetable)) continue
    shapes.push({
      route_short_name: route.route_short_name,
      route_long_name: route.route_long_name,
      route_id: route.route_id,
      route_type: route.route_type,
      route_color: route.route_color,
      stop_time: routeStopTimes(index, route, dayType, hour),
      timetable: route.timetable,
    })
  }
  const sorted = [...trips].sort()
  return {
    stop_id: stop.stop_id,
    stop_name: stop.stop_name,
    stop_desc: stop.stop_desc,
    stop_lat: stop.stop_lat,
    stop_lon: stop.stop_lon,
    location_type: stop.location_type,
    stop_code: stop.stop_code,
    outgoing_trip_ids: sorted.filter((t) => t.endsWith(OUTGOING_SUFFIX)),
    incoming_trip_ids: sorted.filter((t) => t.endsWith(INCOMING_SUFFIX)),
    shapes_info: shapes,
  }
}

export function buildHourlyOffsets(index: NetworkIndex, route: NetworkRoute, dayType: DayType): Record<string, HourlyStopOffsets> {
  const out: Record<string, HourlyStopOffsets> = {}
  for (let hour = 0; hour < 24; hour++) {
    const byTrip = new Map<string, StopTime[]>()
    for (const st of routeStopTimes(index, route, dayType, hour)) {
      const list = byTrip.get(st.trip_id) ?? []
      list.push(st)
      byTrip.set(st.trip_id, list)
    }
    for (const [tripId, trip] of byTrip) {
      trip.sort((a, b) => a.stop_sequence - b.stop_sequence)
      const entry = out[tripId] ?? (out[tripId] = {stop_ids: trip.map((st) => st.stop_id), hourly_offset_seconds: {}})
      let cumulative = 0
      entry.hourly_offset_seconds[String(hour)] = trip.map((st) => (cumulative += st.offset_arrival_time))
    }
  }
  return out
}

export const network = shallowRef<NetworkIndex | null>(null)

const CACHE_NAME = 'network-bundle'
const BUNDLE_URL = '/api/network'
const META_KEY = 'network:meta'
const MAX_AGE_MS = 12 * 60 * 60_000
const CHECK_EVERY_MS = 15 * 60_000

type Meta = {version: string, static_version: string, fetchedAt: number}

function readMeta(): Meta | null {
  try {
    return JSON.parse(localStorage.getItem(META_KEY) ?? 'null')
  } catch {
    return null
  }
}

function writeMeta(meta: Meta) {
  try {
    localStorage.setItem(META_KEY, JSON.stringify(meta))
  } catch { /* noop */ }
}

async function openCache(): Promise<Cache | null> {
  try {
    return typeof caches === 'undefined' ? null : await caches.open(CACHE_NAME)
  } catch {
    return null
  }
}

function use(bundle: NetworkBundle) {
  if (!Array.isArray(bundle?.routes) || !Array.isArray(bundle?.stops)) return
  network.value = new NetworkIndex(bundle)
}

let lastCheck = 0
let checking: Promise<void> | null = null

export function refreshNetwork(now = false): Promise<void> {
  if (checking) return checking
  if (!now && Date.now() - lastCheck < CHECK_EVERY_MS) return Promise.resolve()
  lastCheck = Date.now()
  checking = (async () => {
    try {
      const meta = readMeta()
      const response = await fetch(`${BUNDLE_URL}/version`, {cache: 'no-store'})
      if (!response.ok) return
      const latest = await response.json() as NetworkVersion
      const fresh = meta && network.value && meta.static_version === latest.static_version && Date.now() - meta.fetchedAt < MAX_AGE_MS
      if (fresh) return
      if (meta && network.value && meta.version === latest.version) {
        writeMeta({...meta, fetchedAt: Date.now()})
        return
      }
      const bundleResponse = await fetch(BUNDLE_URL, {cache: 'no-cache'})
      if (!bundleResponse.ok) return
      const cache = await openCache()
      await cache?.put(BUNDLE_URL, bundleResponse.clone())
      const bundle = await bundleResponse.json() as NetworkBundle
      use(bundle)
      writeMeta({version: bundle.version, static_version: bundle.static_version, fetchedAt: Date.now()})
    } catch (e) {
      console.warn('network bundle:', e)
    } finally {
      checking = null
    }
  })()
  return checking
}

async function loadStored() {
  try {
    const cached = await (await openCache())?.match(BUNDLE_URL)
    if (cached) use(await cached.json() as NetworkBundle)
  } catch { /* noop */ }
}

export const networkLoaded: Promise<void> = typeof window === 'undefined' ? Promise.resolve() : loadStored()

export async function startNetwork() {
  await networkLoaded
  const saveData = (navigator as Navigator & {connection?: {saveData?: boolean}}).connection?.saveData
  if (!network.value && saveData) return
  const later = (fn: () => void) => ('requestIdleCallback' in window ? window.requestIdleCallback(fn, {timeout: 5000}) : setTimeout(fn, 3000))
  later(() => void refreshNetwork(true))
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) void refreshNetwork()
  })
}

export function decodePolyline(encoded: string, precision = 6): [number, number][] {
  const factor = 10 ** precision
  const points: [number, number][] = []
  let index = 0, lat = 0, lon = 0
  while (index < encoded.length) {
    for (let coordinate = 0; coordinate < 2; coordinate++) {
      let shift = 0, result = 0, byte: number
      do {
        byte = encoded.charCodeAt(index++) - 63
        result |= (byte & 0x1f) << shift
        shift += 5
      } while (byte >= 0x20)
      const delta = result & 1 ? ~(result >> 1) : result >> 1
      if (coordinate === 0) lat += delta
      else lon += delta
    }
    points.push([lat / factor, lon / factor])
  }
  return points
}
