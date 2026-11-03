import {cacheKey} from '@/utils/city.ts'
import {cityTime} from '@/utils/cityClock.ts'

export type DayPart = 'morning' | 'afternoon' | 'evening' | 'night'
export type When = {part: DayPart; weekend: boolean}

export type Usage = {counts: Record<string, number>; last: number; dismissed?: boolean}
export type UsageLog = Record<string, Usage>
export type UsageKind = 'lines' | 'groups'

export function dayPart(hour: number): DayPart {
  if (hour < 5) return 'night'
  if (hour < 12) return 'morning'
  if (hour < 18) return 'afternoon'
  return 'evening'
}

export function whenOf(at: number): When {
  const {hour, weekday} = cityTime(at)
  return {part: dayPart(hour), weekend: weekday === 0 || weekday === 6}
}

const slot = (when: When) => `${when.part}/${when.weekend ? 'weekend' : 'weekday'}`

const storageKey = (kind: UsageKind) => cacheKey(`usage:${kind}`)

export function readUsage(kind: UsageKind): UsageLog {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(storageKey(kind)) ?? 'null')
    return raw && typeof raw === 'object' && !Array.isArray(raw) ? (raw as UsageLog) : {}
  } catch {
    return {}
  }
}

function writeUsage(kind: UsageKind, log: UsageLog) {
  try {
    localStorage.setItem(storageKey(kind), JSON.stringify(log))
  } catch {
    /* noop */
  }
}

export function recordUse(kind: UsageKind, key: string, at = Date.now()) {
  const log = readUsage(kind)
  const usage = log[key] ?? {counts: {}, last: 0}
  const s = slot(whenOf(at))
  usage.counts[s] = (usage.counts[s] ?? 0) + 1
  usage.last = at
  log[key] = usage
  writeUsage(kind, log)
}

export function dismissUse(kind: UsageKind, key: string) {
  const log = readUsage(kind)
  if (!log[key]) return
  log[key].dismissed = true
  writeUsage(kind, log)
}

export const totalUses = (usage: Usage | undefined) => Object.values(usage?.counts ?? {}).reduce((a, b) => a + b, 0)

export function lineUse(log: UsageLog, routeId: number): Usage | undefined {
  const [a, b] = [log[`${routeId}/0`], log[`${routeId}/1`]]
  if (!a || !b) return a ?? b
  const counts = {...a.counts}
  for (const [s, n] of Object.entries(b.counts)) counts[s] = (counts[s] ?? 0) + n
  return {counts, last: Math.max(a.last, b.last)}
}

export function compareUse(a: Usage | undefined, b: Usage | undefined, now: When): number {
  const s = slot(now)
  return (b?.counts[s] ?? 0) - (a?.counts[s] ?? 0) || totalUses(b) - totalUses(a) || (b?.last ?? 0) - (a?.last ?? 0)
}
