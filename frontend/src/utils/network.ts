import {cityTime} from '@/utils/cityClock.ts'
import {shallowRef, watch} from 'vue'
import type {HourlyStopOffsets, Route, ShapeInfo, Stop, StopInfo, StopTime} from '@/types/tranzy.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX} from '@/types/tranzy.ts'
import type {Timetable} from '@/types/ctp.ts'
import {type DayNote, hasTimetableEntries, scheduleDayOn, setScheduleDays} from '@/utils/time.ts'
import {apiPath, cacheKey, currentCity} from '@/utils/city.ts'
import {byModeThenNumber, groupByMode, type Mode, modeOf} from '@/utils/modes.ts'

export type NetworkTrip = {
  trip_id: string
  stop_ids: number[]
  stop_sequences: number[]
  shape_hash?: string
}

export type NetworkRoute = Route & {
  route_colorful?: string
  slug?: string
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
  rules?: {late_grace_min: number}
  schedule_days?: Record<string, DayType>
  day_notes?: Record<string, DayNote>
}

export type NetworkVersion = Pick<NetworkBundle, 'version' | 'static_version' | 'generated_at'>

export type DayType = 'weekday' | 'saturday' | 'sunday'

export class NetworkIndex {
  readonly stopById = new Map<number, Stop>()
  readonly routeById = new Map<number, NetworkRoute>()
  readonly routeByShortName = new Map<string, NetworkRoute>()
  readonly stationBySlug = new Map<string, number>()
  readonly routeBySlug = new Map<string, number>()
  readonly routes: Route[]

  constructor(readonly bundle: NetworkBundle) {
    for (const s of bundle.stops) {
      this.stopById.set(s.stop_id, s)
      if (s.slug) this.stationBySlug.set(s.slug, stationOf(s))
    }
    for (const r of bundle.routes) {
      this.routeById.set(r.route_id, r)
      this.routeByShortName.set(r.route_short_name, r)
      if (r.slug) this.routeBySlug.set(r.slug, r.route_id)
    }
    this.routes = bundle.routes.map(({route_id, agency_id, route_short_name, route_long_name, route_type, route_desc, route_color}) => ({
      route_id,
      agency_id,
      route_short_name,
      route_long_name,
      route_type,
      route_desc,
      route_color,
    }))
  }

  // A timetable run this many minutes late is still coming; shown to the minute, so one at least.
  get lateGraceMin(): number {
    return Math.max(1, this.bundle.rules?.late_grace_min ?? 1)
  }

  shapeHash(tripId: string): string | undefined {
    const route = this.routeById.get(Number(tripId.split('_')[0]))
    return route?.trips.find((t) => t.trip_id === tripId)?.shape_hash
  }

  private stationIndex?: {platforms: Map<number, Stop[]>; looks: Map<number, StationLook>}
  private modeColorIndex?: Map<Mode, string>

  modeColor(mode: Mode): string | undefined {
    return (this.modeColorIndex ??= modeColors(this.bundle.routes)).get(mode)
  }

  private get stations() {
    if (this.stationIndex) return this.stationIndex
    const platforms = new Map<number, Stop[]>()
    for (const s of this.bundle.stops) {
      const id = stationOf(s)
      const list = platforms.get(id)
      if (list) list.push(s)
      else platforms.set(id, [s])
    }
    const routes = new Map<number, NetworkRoute[]>()
    // Only the lines a stop page lists.
    for (const route of [...this.bundle.routes].sort(byModeThenNumber)) {
      if (!route.timetable || !hasTimetableEntries(route.timetable)) continue
      const stations = new Set<number>()
      for (const trip of route.trips) {
        for (const stopId of trip.stop_ids) {
          const stop = this.stopById.get(stopId)
          if (stop) stations.add(stationOf(stop))
        }
      }
      for (const id of stations) {
        const list = routes.get(id)
        if (list) list.push(route)
        else routes.set(id, [route])
      }
    }
    const looks = new Map<number, StationLook>()
    for (const [id, lines] of routes) {
      const groups = groupByMode(lines).map(({mode, lines: routes}) => {
        const colors = new Set(routes.map((l) => l.route_color))
        return {mode, color: colors.size === 1 ? [...colors][0]! : this.modeColor(mode)!, routes}
      })
      looks.set(id, {mode: groups[0]!.mode, color: groups[0]!.color, routes: lines, groups})
    }
    return (this.stationIndex = {platforms, looks})
  }

  stationId(stopId: number): number {
    const stop = this.stopById.get(stopId)
    return stop ? stationOf(stop) : stopId
  }

  look(stopId: number): StationLook | undefined {
    const stop = this.stopById.get(stopId)
    return stop && this.stations.looks.get(stationOf(stop))
  }

  platforms(stopId: number): Stop[] {
    const stop = this.stopById.get(stopId)
    if (!stop) return []
    const all = this.stations.platforms.get(stationOf(stop)) ?? [stop]
    return [stop, ...all.filter((s) => s.stop_id !== stopId)]
  }

  oneStopPerStation<T extends Stop>(stops: T[]): T[] {
    const seen = new Set<number>()
    return stops.filter((s) => {
      const id = stationOf(this.stopById.get(s.stop_id) ?? s)
      if (seen.has(id)) return false
      seen.add(id)
      return true
    })
  }
}

export const stationOf = (stop: Stop): number => stop.station_id || stop.stop_id

export type ModeLines = {mode: Mode; color: string; routes: NetworkRoute[]}

export type StationLook = ModeLines & {groups: ModeLines[]}

function modeColors(routes: NetworkRoute[]): Map<Mode, string> {
  const counts = new Map<Mode, Map<string, number>>()
  for (const route of [...routes].sort(byModeThenNumber)) {
    const mode = modeOf(route.route_type)
    const byColor = counts.get(mode) ?? new Map<string, number>()
    byColor.set(route.route_color, (byColor.get(route.route_color) ?? 0) + 1)
    counts.set(mode, byColor)
  }
  const out = new Map<Mode, string>()
  for (const [mode, byColor] of counts) {
    let best = ''
    for (const [color, n] of byColor) if (!best || n > byColor.get(best)!) best = color
    out.set(mode, best)
  }
  return out
}

// The kind of day and the hour in the city, which pick the learned offsets.
export function dayTypeAndHour(at: Date = new Date()): {dayType: DayType; hour: number} {
  const {date, weekday, hour} = cityTime(at)
  const special = scheduleDayOn(date)
  if (special) return {dayType: special, hour}
  return {dayType: weekday === 6 ? 'saturday' : weekday === 0 ? 'sunday' : 'weekday', hour}
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
  const {dayType, hour} = dayTypeAndHour(at)
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
      late_grace_min: index.lateGraceMin,
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
    late_grace_min: index.lateGraceMin,
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
const bundleUrl = () => apiPath('network')
const metaKey = () => cacheKey('network:meta')
const MAX_AGE_MS = 12 * 60 * 60_000
const CHECK_EVERY_MS = 15 * 60_000

type Meta = {version: string; static_version: string; fetchedAt: number}

function readMeta(): Meta | null {
  try {
    return JSON.parse(localStorage.getItem(metaKey()) ?? 'null')
  } catch {
    return null
  }
}

function writeMeta(meta: Meta) {
  try {
    localStorage.setItem(metaKey(), JSON.stringify(meta))
  } catch {
    /* noop */
  }
}

async function openCache(): Promise<Cache | null> {
  try {
    return typeof caches === 'undefined' ? null : await caches.open(CACHE_NAME)
  } catch {
    return null
  }
}

function use(bundle: NetworkBundle, city: string) {
  if (city !== currentCity.value || !Array.isArray(bundle?.routes) || !Array.isArray(bundle?.stops)) return
  setScheduleDays(bundle.schedule_days, bundle.day_notes)
  network.value = new NetworkIndex(bundle)
}

let lastCheck = 0
let checking: Promise<void> | null = null

async function fetchBundle(city: string) {
  const url = bundleUrl()
  try {
    const meta = readMeta()
    const response = await fetch(`${url}/version`, {cache: 'no-store'})
    if (!response.ok) return
    const latest = (await response.json()) as NetworkVersion
    if (city !== currentCity.value) return
    const fresh = meta && network.value && meta.static_version === latest.static_version && Date.now() - meta.fetchedAt < MAX_AGE_MS
    if (fresh) return
    if (meta && network.value && meta.version === latest.version) {
      writeMeta({...meta, fetchedAt: Date.now()})
      return
    }
    const bundleResponse = await fetch(url, {cache: 'no-cache'})
    if (!bundleResponse.ok) return
    const cache = await openCache()
    await cache?.put(url, bundleResponse.clone())
    const bundle = (await bundleResponse.json()) as NetworkBundle
    if (city !== currentCity.value) return
    use(bundle, city)
    writeMeta({version: bundle.version, static_version: bundle.static_version, fetchedAt: Date.now()})
  } catch (e) {
    console.warn('network bundle:', e)
  }
}

export function refreshNetwork(now = false): Promise<void> {
  if (checking) return checking
  if (!now && Date.now() - lastCheck < CHECK_EVERY_MS) return Promise.resolve()
  lastCheck = Date.now()
  const check = fetchBundle(currentCity.value).finally(() => {
    if (checking === check) checking = null
  })
  checking = check
  return check
}

async function loadStored() {
  const city = currentCity.value
  try {
    const cached = await (await openCache())?.match(bundleUrl())
    if (cached) use((await cached.json()) as NetworkBundle, city)
  } catch {
    /* noop */
  }
}

let loaded: Promise<void> = typeof window === 'undefined' ? Promise.resolve() : loadStored()

export const networkLoaded = () => loaded

export async function loadedNetwork() {
  await networkLoaded()
  if (!network.value) await refreshNetwork(true)
  return network.value
}

watch(
  currentCity,
  () => {
    network.value = null
    lastCheck = 0
    checking = null
    loaded = loadStored()
    void loaded.then(() => refreshNetwork(true))
  },
  {flush: 'sync'},
)

export async function startNetwork() {
  await networkLoaded()
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
  let index = 0,
    lat = 0,
    lon = 0
  while (index < encoded.length) {
    for (let coordinate = 0; coordinate < 2; coordinate++) {
      let shift = 0,
        result = 0,
        byte: number
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
