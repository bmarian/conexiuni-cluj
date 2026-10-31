// The record rules of backend/devicesync/protocol.md §4: a strict reader and the canonical writer.

import {fromB64, SPKI_SIZE, validPublicKey} from '@/utils/sync/crypto.ts'

export const CITY_CAP = 50
export const NEWS_CAP = 200
export const DEVICE_CAP = 10
export const RECENT_CAP = 10
const MAX_INT = 2 ** 31 - 1

export type Place = {name: string; label: string; lat: number; lon: number; origin?: {name: string; lat: number; lon: number}}
export type Line = {route: number; dir: '0' | '1'}
export type CityRecord = {stops: number[]; lines: Line[]; places: Place[]; followed: string[]}
export type NewsRecord = {seen: number[]}
export type RecentItem = {kind: 'stop' | 'line'; id: number} | {kind: 'place'; place: Place}
export type RecentsRecord = {recent: RecentItem[]}
export type DeviceKind = 'web' | 'android' | 'wear'
export type DeviceEntry = {id: string; kind: DeviceKind; label: string; pub: string}
export type DevicesRecord = {devices: DeviceEntry[]}
export type Theme = 'light' | 'dark' | 'system'
export type SettingsRecord = {
  theme: Theme
  locale: 'ro' | 'en'
  autoCenterOnMe: boolean
  autoFitMap: boolean
  showVehicleExtras: boolean
  greenFriday: boolean
}

export type OptionsRecord = {colorfulLines: boolean; city: string; journeyVoice: boolean | null; fullScreenAlarms: boolean | null}

// newer: a later version this device leaves alone.
export type Read<T> = {ok: true; value: T} | {ok: false; newer: boolean}

class Refused extends Error {}

const refuse = (why: string): never => {
  throw new Refused(why)
}

export const emptyCity = (): CityRecord => ({stops: [], lines: [], places: [], followed: []})
export const emptyNews = (): NewsRecord => ({seen: []})
export const emptyRecents = (): RecentsRecord => ({recent: []})

export const validId = (id: number) => Number.isInteger(id) && id >= 1 && id <= MAX_INT

export const newestSeen = (ids: number[]) => [...ids].sort((a, b) => b - a).slice(0, NEWS_CAP)

export function forbidden(cp: number): boolean {
  return (
    cp <= 0x1f ||
    (cp >= 0x7f && cp <= 0x9f) ||
    cp === 0x061c ||
    cp === 0x200b ||
    cp === 0x200e ||
    cp === 0x200f ||
    (cp >= 0x2028 && cp <= 0x202e) ||
    cp === 0x2060 ||
    (cp >= 0x2066 && cp <= 0x2069) ||
    cp === 0xfeff ||
    cp === 0xfffd ||
    cp === 0xfffe ||
    cp === 0xffff ||
    (cp >= 0xd800 && cp <= 0xdfff)
  )
}

export function blankEnd(cp: number): boolean {
  return cp === 0x20 || cp === 0xa0 || cp === 0x1680 || (cp >= 0x2000 && cp <= 0x200a) || cp === 0x202f || cp === 0x205f || cp === 0x3000
}

export function validText(value: unknown, min: number, max: number): value is string {
  if (typeof value !== 'string') return false
  const points = [...value].map((ch) => ch.codePointAt(0)!)
  if (points.length < min || points.length > max || points.some(forbidden)) return false
  return !points.length || (!blankEnd(points[0]!) && !blankEnd(points.at(-1)!))
}

const text = (value: unknown, min: number, max: number): string => (validText(value, min, max) ? value : refuse('text'))

function integer(value: unknown, lo: number, hi: number): number {
  return typeof value === 'number' && Number.isInteger(value) && value >= lo && value <= hi ? value : refuse('integer')
}

export const LAT = [43.6, 48.3] as const
export const LON = [20.2, 29.8] as const

export const roundCoordinate = (x: number) => Math.round(x * 1e6) / 1e6

function coordinate(value: unknown, [lo, hi]: readonly [number, number]): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= lo && value <= hi && roundCoordinate(value) === value ? value : refuse('coordinate')
}

const bool = (value: unknown): boolean => (typeof value === 'boolean' ? value : refuse('boolean'))

function list(value: unknown, cap: number): unknown[] {
  return Array.isArray(value) && value.length <= cap ? value : refuse('list')
}

function object(value: unknown, ...keys: string[]): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) refuse('object')
  const record = value as Record<string, unknown>
  const own = Object.keys(record)
  return own.length === keys.length && keys.every((k) => own.includes(k)) ? record : refuse('keys')
}

function unique<T>(items: T[], id: (item: T) => string) {
  if (new Set(items.map(id)).size !== items.length) refuse('duplicate')
}

export const stopId = (id: number) => String(id)
export const lineId = (l: Line) => `${l.route}/${l.dir}`
export const placeId = (p: Place) => [p.lat, p.lon, p.origin?.lat ?? '', p.origin?.lon ?? ''].join(',')
export const followedId = (name: string) => name
export const deviceId = (d: DeviceEntry) => d.id
export const recentId = (r: RecentItem) => (r.kind === 'place' ? `place:${placeId(r.place)}` : `${r.kind}:${r.id}`)

// Parses, checks the rules and the canonical form; the version check comes first, so a later shape is "newer", not refused.
function read<T>(raw: string, parse: (record: unknown) => T, write: (value: T) => string): Read<T> {
  try {
    const record: unknown = JSON.parse(raw)
    const v = typeof record === 'object' && record !== null ? (record as {v?: unknown}).v : undefined
    if (integer(v, 1, MAX_INT) > 1) return {ok: false, newer: true}
    const value = parse(record)
    return write(value) === raw ? {ok: true, value} : {ok: false, newer: false}
  } catch {
    return {ok: false, newer: false}
  }
}

function parsePlace(f: unknown[]): Place {
  if (f.length !== 4 && f.length !== 7) refuse('place')
  const place: Place = {name: text(f[0], 1, 100), label: text(f[1], 0, 40), lat: coordinate(f[2], LAT), lon: coordinate(f[3], LON)}
  if (f.length === 7) place.origin = {name: text(f[4], 1, 100), lat: coordinate(f[5], LAT), lon: coordinate(f[6], LON)}
  return place
}

const placeFields = (p: Place) => (p.origin ? [p.name, p.label, p.lat, p.lon, p.origin.name, p.origin.lat, p.origin.lon] : [p.name, p.label, p.lat, p.lon])

function parseCity(record: unknown): CityRecord {
  const m = object(record, 'v', 'stops', 'lines', 'places', 'followed')
  const stops = list(m.stops, CITY_CAP).map((s) => integer(s, 1, MAX_INT))
  const lines = list(m.lines, CITY_CAP).map((l): Line => {
    const pair = list(l, 2)
    if (pair.length !== 2 || (pair[1] !== '0' && pair[1] !== '1')) refuse('line')
    return {route: integer(pair[0], 1, MAX_INT), dir: pair[1] as '0' | '1'}
  })
  const places = list(m.places, CITY_CAP).map((p) => parsePlace(list(p, 7)))
  const followed = list(m.followed, CITY_CAP).map((f) => text(f, 1, 16))
  unique(stops, stopId)
  unique(lines, lineId)
  unique(places, placeId)
  unique(followed, followedId)
  return {stops, lines, places, followed}
}

export function writeCity(r: CityRecord): string {
  return JSON.stringify({
    v: 1,
    stops: r.stops,
    lines: r.lines.map((l) => [l.route, l.dir]),
    places: r.places.map(placeFields),
    followed: r.followed,
  })
}

export const readCity = (raw: string) => read(raw, parseCity, writeCity)

function parseNews(record: unknown): NewsRecord {
  const seen = list(object(record, 'v', 'seen').seen, NEWS_CAP).map((s) => integer(s, 1, MAX_INT))
  if (seen.some((id, i) => i > 0 && id >= seen[i - 1]!)) refuse('order')
  return {seen}
}

export const writeNews = (r: NewsRecord) => JSON.stringify({v: 1, seen: r.seen})

export const readNews = (raw: string) => read(raw, parseNews, writeNews)

function parseRecents(record: unknown): RecentsRecord {
  const recent = list(object(record, 'v', 'recent').recent, RECENT_CAP).map((item): RecentItem => {
    const f = list(item, 8)
    if (f[0] === 'place') return {kind: 'place', place: parsePlace(f.slice(1))}
    if ((f[0] !== 'stop' && f[0] !== 'line') || f.length !== 2) refuse('recent')
    return {kind: f[0] as 'stop' | 'line', id: integer(f[1], 1, MAX_INT)}
  })
  unique(recent, recentId)
  return {recent}
}

export function writeRecents(r: RecentsRecord): string {
  return JSON.stringify({v: 1, recent: r.recent.map((item) => (item.kind === 'place' ? ['place', ...placeFields(item.place)] : [item.kind, item.id]))})
}

export const readRecents = (raw: string) => read(raw, parseRecents, writeRecents)

const KINDS: DeviceKind[] = ['web', 'android', 'wear']

// Base64url of 16 bytes: the last character carries two bits.
const ID_TEXT = /^[A-Za-z0-9_-]{21}[AQgw]$/

function parseDevices(record: unknown): DevicesRecord {
  const m = object(record, 'v', 'devices')
  const devices = list(m.devices, DEVICE_CAP).map((item): DeviceEntry => {
    const d = object(item, 'id', 'kind', 'label', 'pub')
    if (typeof d.id !== 'string' || !ID_TEXT.test(d.id)) refuse('id')
    if (!KINDS.includes(d.kind as DeviceKind)) refuse('kind')
    if (typeof d.pub !== 'string') refuse('key')
    return {id: d.id as string, kind: d.kind as DeviceKind, label: text(d.label, 1, 40), pub: d.pub as string}
  })
  unique(devices, deviceId)
  return {devices}
}

export function writeDevices(r: DevicesRecord): string {
  return JSON.stringify({v: 1, devices: r.devices.map((d) => ({id: d.id, kind: d.kind, label: d.label, pub: d.pub}))})
}

// Async: each key must be a P-256 point, which only WebCrypto can check.
export async function readDevices(raw: string): Promise<Read<DevicesRecord>> {
  const result = read(raw, parseDevices, writeDevices)
  if (!result.ok) return result
  for (const d of result.value.devices) {
    const key = fromB64(d.pub, SPKI_SIZE)
    if (!key || !(await validPublicKey(key))) return {ok: false, newer: false}
  }
  return result
}

function parseSettings(record: unknown): SettingsRecord {
  const m = object(record, 'v', 'theme', 'locale', 'autoCenterOnMe', 'autoFitMap', 'showVehicleExtras', 'regions')
  if (m.theme !== 'light' && m.theme !== 'dark' && m.theme !== 'system') refuse('theme')
  if (m.locale !== 'ro' && m.locale !== 'en') refuse('locale')
  const cluj = object(object(m.regions, 'cluj').cluj, 'showGreenFriday')
  return {
    theme: m.theme as Theme,
    locale: m.locale as 'ro' | 'en',
    autoCenterOnMe: bool(m.autoCenterOnMe),
    autoFitMap: bool(m.autoFitMap),
    showVehicleExtras: bool(m.showVehicleExtras),
    greenFriday: bool(cluj.showGreenFriday),
  }
}

export function writeSettings(r: SettingsRecord): string {
  return JSON.stringify({
    v: 1,
    theme: r.theme,
    locale: r.locale,
    autoCenterOnMe: r.autoCenterOnMe,
    autoFitMap: r.autoFitMap,
    showVehicleExtras: r.showVehicleExtras,
    regions: {cluj: {showGreenFriday: r.greenFriday}},
  })
}

export const readSettings = (raw: string) => read(raw, parseSettings, writeSettings)

const maybeBool = (value: unknown): boolean | null => (value === null ? null : bool(value))

function parseOptions(record: unknown): OptionsRecord {
  const m = object(record, 'v', 'colorfulLines', 'city', 'journeyVoice', 'fullScreenAlarms')
  return {colorfulLines: bool(m.colorfulLines), city: text(m.city, 0, 40), journeyVoice: maybeBool(m.journeyVoice), fullScreenAlarms: maybeBool(m.fullScreenAlarms)}
}

export function writeOptions(r: OptionsRecord): string {
  return JSON.stringify({v: 1, colorfulLines: r.colorfulLines, city: r.city, journeyVoice: r.journeyVoice, fullScreenAlarms: r.fullScreenAlarms})
}

export const readOptions = (raw: string) => read(raw, parseOptions, writeOptions)
