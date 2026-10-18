import type {Timetable} from '@/types/ctp.ts'
import type {HourlyStopOffsets} from '@/types/tranzy.ts'
import type {DayType} from '@/utils/network.ts'
import {scheduleDayOn, timeStringToMinutes} from '@/utils/time.ts'

// Earlier times belong to the previous service day.
const NIGHT_END_MIN = 180
const NEAREST_WITHIN_MIN = 20
export const DAY_TYPES: DayType[] = ['weekday', 'saturday', 'sunday']
export const MAX_REMINDERS = 10

export type Slots = Partial<Record<DayType, number>>

export interface Reminder {
  id: string
  stopId: number
  stopName: string
  stopLat?: number
  stopLon?: number
  routeId: number
  tripId: string
  line: string
  color: string
  destination: string
  /** Minutes after midnight, past 1440 after it; null: last bus. */
  time: number | null
  /** ISO weekdays; empty: once. */
  days: number[]
  /** YYYY-MM-DD, once only. */
  date: string | null
  slots: Slots
  /** Minutes to the stop from where it was set; null when unknown. */
  walk?: number | null
  /** Epoch ms when the walk was measured. */
  walkAt?: number
}

const scheduleFor = (timetable: Timetable, dayType: DayType) =>
  dayType === 'weekday' ? timetable.weekdays : dayType === 'saturday' ? timetable.saturday : timetable.sunday

export function stopTimesByDayType(
  timetable: Timetable,
  outgoing: boolean,
  stopId: number,
  hourly: Partial<Record<DayType, HourlyStopOffsets | undefined>>,
  fallbackOffsetSeconds: number,
): Partial<Record<DayType, number[]>> {
  const out: Partial<Record<DayType, number[]>> = {}
  for (const dayType of DAY_TYPES) {
    const entries = scheduleFor(timetable, dayType)?.entries ?? []
    const trip = hourly[dayType]
    const index = trip?.stop_ids.indexOf(stopId) ?? -1
    const times = entries
      .map((entry) => timeStringToMinutes((outgoing ? entry.departure_in : entry.departure_out) ?? ''))
      .filter((m): m is number => m !== null)
      .map((departure) => {
        const seconds = index >= 0 ? trip?.hourly_offset_seconds[String(Math.floor(departure / 60) % 24)]?.[index] : undefined
        const at = Math.round(departure + (seconds ?? fallbackOffsetSeconds) / 60)
        return at < NIGHT_END_MIN ? at + 1440 : at
      })
      .sort((a, b) => a - b)
    if (times.length) out[dayType] = times
  }
  return out
}

export function pickSlot(times: number[], time: number | null): number | null {
  if (!times.length) return null
  if (time === null) return Math.max(...times)
  const nearest = times.reduce((best, m) => (Math.abs(m - time) < Math.abs(best - time) ? m : best))
  return Math.abs(nearest - time) <= NEAREST_WITHIN_MIN ? nearest : null
}

export function resolveSlots(times: Partial<Record<DayType, number[]>>, time: number | null): Slots {
  const slots: Slots = {}
  for (const dayType of DAY_TYPES) {
    const slot = pickSlot(times[dayType] ?? [], time)
    if (slot !== null) slots[dayType] = slot
  }
  return slots
}

const CLUJ = 'Europe/Bucharest'
const DATE_PARTS = new Intl.DateTimeFormat('en-GB', {
  timeZone: CLUJ, year: 'numeric', month: '2-digit', day: '2-digit', hour: 'numeric', minute: 'numeric', hourCycle: 'h23',
})

export function clujDate(ms: number): string {
  const parts = DATE_PARTS.formatToParts(new Date(ms))
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? ''
  return `${get('year')}-${get('month')}-${get('day')}`
}

function clujOffsetMs(ms: number): number {
  const parts = DATE_PARTS.formatToParts(new Date(ms))
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? 0)
  const asUtc = Date.UTC(get('year'), get('month') - 1, get('day'), get('hour') % 24, get('minute'))
  return asUtc - Math.floor(ms / 60_000) * 60_000
}

export function clujAt(date: string, minutes: number): number {
  const [y, m, d] = date.split('-').map(Number)
  const local = Date.UTC(y!, m! - 1, d!) + minutes * 60_000
  let guess = local - clujOffsetMs(local)
  guess = local - clujOffsetMs(guess)
  return guess
}

export function addDays(date: string, days: number): string {
  const [y, m, d] = date.split('-').map(Number)
  const t = new Date(Date.UTC(y!, m! - 1, d! + days))
  return t.toISOString().slice(0, 10)
}

export function isoWeekday(date: string): number {
  const [y, m, d] = date.split('-').map(Number)
  return ((new Date(Date.UTC(y!, m! - 1, d!)).getUTCDay() + 6) % 7) + 1
}

export function dayTypeOf(date: string): DayType {
  const special = scheduleDayOn(date)
  if (special) return special
  const day = isoWeekday(date)
  return day === 6 ? 'saturday' : day === 7 ? 'sunday' : 'weekday'
}

// Starts a day back: late buses run past midnight.
export function nextOccurrence(slots: Slots, days: number[], date: string | null, afterMs: number): number | null {
  const today = clujDate(afterMs)
  for (let i = -1; i <= 8; i++) {
    const day = addDays(today, i)
    if (date !== null && day !== date) continue
    if (date === null && !days.includes(isoWeekday(day))) continue
    const minute = slots[dayTypeOf(day)]
    if (minute === undefined) continue
    const at = clujAt(day, minute)
    if (at > afterMs) return at
  }
  return null
}

export const nextOf = (reminder: Reminder, afterMs: number) => nextOccurrence(reminder.slots, reminder.days, reminder.date, afterMs)

/** The first of the timetable [times] after [afterMs]: its service day and its minute in that day. */
export function nextDeparture(times: Partial<Record<DayType, number[]>>, afterMs: number): {date: string, minute: number} | null {
  const today = clujDate(afterMs)
  let first: {date: string, minute: number} | null = null
  let firstAt = Infinity
  for (let i = -1; i <= 8; i++) {
    const date = addDays(today, i)
    for (const minute of times[dayTypeOf(date)] ?? []) {
      const at = clujAt(date, minute)
      if (at > afterMs && at < firstAt) {
        first = {date, minute}
        firstAt = at
      }
    }
  }
  return first
}

export function formatSlot(minutes: number): string {
  const m = ((minutes % 1440) + 1440) % 1440
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`
}
