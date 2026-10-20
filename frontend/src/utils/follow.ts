import {cityClock, cityMinutes} from '@/utils/cityClock.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {minutesUntil} from '@/utils/arrivals.ts'
import {WALKING_SPEED_M_PER_MIN} from '@/utils/trips.ts'

export const SOON_MINUTES = 2
export const MAX_FOLLOWS = 10
export const GIVE_UP_MS = 90 * 60_000
export const LEAVE_BUFFER_MIN = 5
// How early to go when the walk isn't known.
export const NO_LOCATION_LEAD_MIN = 20
export const CLOCK_FROM_MINUTES = 60
const WALK_DETOUR = 1.25
const AT_STOP_METERS = 60

export interface Followed {
  id: string
  stopId: number
  stopName: string
  stopLat: number
  stopLon: number
  routeId: number
  tripId: string
  line: string
  color: string
  destination: string
  startedAt: number
  /** Reminder's bus, epoch ms. */
  targetAt?: number
}

export interface FollowReading {
  minutes: number
  live: boolean
  later: number[]
}

export interface FollowProgress {
  lastMinutes: number | null
  alertedSoon: boolean
  arrived: boolean
}

export type FollowAlert = 'soon' | 'arrived'
export type FollowEnd = 'left' | 'none' | 'stopped'

export const freshProgress = (): FollowProgress => ({lastMinutes: null, alertedSoon: false, arrived: false})

export function followId(stopId: number, routeId: number, tripId: string, targetAt?: number): string {
  return targetAt ? `${stopId}:${routeId}:${tripId}@${targetAt}` : `${stopId}:${routeId}:${tripId}`
}

export function walkMinutes(fromLat: number, fromLon: number, toLat: number, toLon: number): number {
  const meters = haversineMeters(fromLat, fromLon, toLat, toLon)
  if (meters < AT_STOP_METERS) return 0
  return Math.ceil(meters * WALK_DETOUR / WALKING_SPEED_M_PER_MIN)
}

export function walkFrom(here: {latitude: number, longitude: number} | null, stopLat?: number, stopLon?: number): number | null {
  if (!here || stopLat === undefined || stopLon === undefined) return null
  return walkMinutes(here.latitude, here.longitude, stopLat, stopLon)
}

// Whole minutes past the city's midnight.
export const cityMinuteOfDay = (ms: number): number => Math.floor(cityMinutes(ms))

export function targetMinutes(scheduled: number[], live: number | null, expected: number, graceMinutes: number): number | null {
  const nearest = (to: number) => scheduled.reduce<number | null>((best, m) =>
    best === null || Math.abs(m - to) < Math.abs(best - to) ? m : best, null)
  const close = nearest(expected)
  const ours = close !== null && Math.abs(close - expected) <= 1 ? close : expected
  if (live !== null && nearest(live) === ours) return live
  if (ours < -graceMinutes) return null
  return Math.max(Math.round(ours), 0)
}

export function targetReading(scheduled: number[], live: number | null, targetAt: number, nowMs: number, graceMinutes: number): FollowReading | null {
  const expected = minutesUntil(cityMinuteOfDay(targetAt), cityMinuteOfDay(nowMs))
  const minutes = targetMinutes(scheduled, live, expected, graceMinutes)
  if (minutes === null) return null
  return {minutes, live: minutes === live, later: scheduled.filter((m) => m > minutes + graceMinutes).slice(0, 2).map(Math.round)}
}

export function leadMinutes(walk: number | null): number | null {
  return walk === null ? null : walk + LEAVE_BUFFER_MIN
}

/** At the stop, or too late to walk there for a bus [minutes] away: just told when it comes. */
export const justNotify = (walk: number | null, minutes: number) =>
  walk === 0 || minutes <= (leadMinutes(walk) ?? NO_LOCATION_LEAD_MIN)

export interface FollowStep {
  progress: FollowProgress
  alerts: FollowAlert[]
  end?: FollowEnd
}

export function stepFollow(
  progress: FollowProgress,
  reading: FollowReading | null,
  options: {target: boolean, nowMs: number, startedAt: number},
): FollowStep {
  const p = {...progress}
  if (options.nowMs - options.startedAt > GIVE_UP_MS) return {progress: p, alerts: [], end: 'stopped'}
  if (!reading) return {progress: p, alerts: [], end: options.target || p.alertedSoon ? 'left' : 'none'}
  const minutes = reading.minutes
  if (p.alertedSoon && minutes > (p.lastMinutes ?? 0) + 2) return {progress: p, alerts: [], end: 'left'}
  // Nothing buzzes on the first reading: the user just asked.
  const opening = p.lastMinutes === null
  const alerts: FollowAlert[] = []
  if (minutes <= SOON_MINUTES && !p.alertedSoon) {
    p.alertedSoon = true
    if (!opening && minutes > 0) alerts.push('soon')
  }
  if (minutes === 0 && !p.arrived) {
    p.arrived = true
    if (!opening) alerts.push('arrived')
  }
  p.lastMinutes = minutes
  return {progress: p, alerts}
}

export const isClockTime = (minutes: number) => minutes >= CLOCK_FROM_MINUTES

export function clockAt(nowMs: number, minutes: number): string {
  return cityClock(nowMs + minutes * 60_000)
}
