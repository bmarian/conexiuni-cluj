// The three-way merge of backend/devicesync/protocol.md §6.
import {
  CITY_CAP,
  type CityRecord,
  DEVICE_CAP,
  type DeviceEntry,
  deviceId,
  followedId,
  lineId,
  newestSeen,
  type NewsRecord,
  type Place,
  placeId,
  type SettingsRecord,
  stopId,
} from '@/utils/sync/records.ts'

// The local value when only this side changed it since base; otherwise the remote one.
export function pick<T>(base: T | undefined, local: T, remote: T, inBase: boolean): T {
  return inBase && local !== base && remote === base ? local : remote
}

type Fields<T> = (base: T | undefined, local: T, remote: T) => T

const sameItem = <T>(_base: T | undefined, _local: T, remote: T) => remote

export function mergeList<T>(base: T[], local: T[], remote: T[], id: (item: T) => string, fields: Fields<T>, cap: number): {merged: T[]; dropped: T[]} {
  const index = (items: T[]) => new Map(items.map((item, i) => [id(item), i]))
  const b = index(base), l = index(local), r = index(remote)
  const kept = (key: string) => (l.has(key) && r.has(key)) || (l.has(key) && !b.has(key)) || (r.has(key) && !b.has(key))
  const orderOf = (items: T[], other: Map<string, number>) => items.map(id).filter((key) => other.has(key)).join('\n')
  const localMoved = orderOf(local, b) !== orderOf(base, l)
  const remoteMoved = orderOf(remote, b) !== orderOf(base, r)
  const [win, other] = localMoved && !remoteMoved ? [local, remote] : [remote, local]
  const keys = win.map(id).filter(kept)
  const placed = new Set(keys)
  for (const key of other.map(id)) {
    if (kept(key) && !placed.has(key)) keys.push(key)
  }
  const merged = keys.map((key) => {
    const li = l.get(key), ri = r.get(key), bi = b.get(key)
    if (li !== undefined && ri !== undefined) return fields(bi === undefined ? undefined : base[bi], local[li], remote[ri])
    return li !== undefined ? local[li]! : remote[ri!]!
  })
  const dropped: T[] = []
  for (let i = merged.length - 1; i >= 0 && merged.length > cap; i--) {
    if (!r.has(id(merged[i]!))) dropped.unshift(...merged.splice(i, 1))
  }
  return {merged, dropped}
}

function mergePlace(base: Place | undefined, local: Place, remote: Place): Place {
  return {...remote, name: pick(base?.name, local.name, remote.name, !!base), label: pick(base?.label, local.label, remote.label, !!base)}
}

function mergeDevice(base: DeviceEntry | undefined, local: DeviceEntry, remote: DeviceEntry): DeviceEntry {
  if (!base) return remote
  return {
    id: remote.id,
    kind: pick(base.kind, local.kind, remote.kind, true),
    label: pick(base.label, local.label, remote.label, true),
    pub: pick(base.pub, local.pub, remote.pub, true),
  }
}

export type CityDropped = {stops: number[]; lines: CityRecord['lines']; places: Place[]; followed: string[]}

export function mergeCity(base: CityRecord, local: CityRecord, remote: CityRecord): {merged: CityRecord; dropped: CityDropped} {
  const stops = mergeList(base.stops, local.stops, remote.stops, stopId, sameItem, CITY_CAP)
  const lines = mergeList(base.lines, local.lines, remote.lines, lineId, sameItem, CITY_CAP)
  const places = mergeList(base.places, local.places, remote.places, placeId, mergePlace, CITY_CAP)
  const followed = mergeList(base.followed, local.followed, remote.followed, followedId, sameItem, CITY_CAP)
  return {
    merged: {stops: stops.merged, lines: lines.merged, places: places.merged, followed: followed.merged},
    dropped: {stops: stops.dropped, lines: lines.dropped, places: places.dropped, followed: followed.dropped},
  }
}

export function mergeNews(base: NewsRecord, local: NewsRecord, remote: NewsRecord): NewsRecord {
  const {merged} = mergeList(base.seen, local.seen, remote.seen, String, sameItem, Infinity)
  return {seen: newestSeen(merged)}
}

export function mergeSettings(base: SettingsRecord | null, local: SettingsRecord, remote: SettingsRecord): SettingsRecord {
  if (!base) return remote
  const out = {...remote}
  for (const key of Object.keys(remote) as (keyof SettingsRecord)[]) {
    ;(out as Record<string, unknown>)[key] = pick(base[key], local[key], remote[key], true)
  }
  return out
}

export function mergeDevices(base: DeviceEntry[], local: DeviceEntry[], remote: DeviceEntry[]): DeviceEntry[] {
  return mergeList(base, local, remote, deviceId, mergeDevice, DEVICE_CAP).merged
}
