import {describe, expect, it} from 'vitest'
import type {Timetable} from '@/types/ctp.ts'
import {
  addDays, clujAt, clujDate, dayTypeOf, formatSlot, isoWeekday, nextDeparture, nextOccurrence, pickSlot, resolveSlots,
  stopTimesByDayType,
} from '@/utils/reminders.ts'
import {setScheduleDays} from '@/utils/time.ts'

const day = (times: string[]) => ({service_name: '', service_start: '', entries: times.map((t) => ({departure_in: t, departure_out: ''}))})
const timetable = {
  route_short_name: '57', route_long_name: 'A - B', in_stop_name: '', out_stop_name: '',
  weekdays: day(['05:30', '22:50', '23:55']),
  saturday: day(['06:00', '22:30']),
  sunday: day([]),
} as Timetable

// Cluj is UTC+3 in summer and UTC+2 in winter.
const ms = (iso: string) => Date.parse(iso)

describe('stopTimesByDayType', () => {
  it('counts times past midnight on from the day before, and leaves out days without buses', () => {
    const times = stopTimesByDayType(timetable, true, 2, {}, 600)
    expect(times.weekday).toEqual([5 * 60 + 40, 23 * 60, 24 * 60 + 5])
    expect(times.saturday).toEqual([6 * 60 + 10, 22 * 60 + 40])
    expect(times.sunday).toBeUndefined()
  })

  it('uses each departure hour’s own travel time to the stop', () => {
    const hourly = {stop_ids: [1, 2], hourly_offset_seconds: {'5': [0, 600], '22': [0, 900], '23': [0, 840]}}
    const times = stopTimesByDayType(timetable, true, 2, {weekday: hourly}, 600)
    expect(times.weekday).toEqual([5 * 60 + 40, 23 * 60 + 5, 24 * 60 + 9])
    expect(times.saturday).toEqual([6 * 60 + 10, 22 * 60 + 40])
  })

  it('reads the other direction’s column for incoming trips', () => {
    const both = {...timetable, weekdays: {...day([]), entries: [{departure_in: '07:00', departure_out: '08:00'}]}} as Timetable
    expect(stopTimesByDayType(both, false, 2, {}, 0).weekday).toEqual([8 * 60])
    expect(stopTimesByDayType(both, true, 2, {}, 0).weekday).toEqual([7 * 60])
  })
})

describe('slots', () => {
  it('takes the day’s last bus, or the one nearest a chosen time', () => {
    const times = stopTimesByDayType(timetable, true, 2, {}, 600)
    expect(resolveSlots(times, null)).toEqual({weekday: 24 * 60 + 5, saturday: 22 * 60 + 40})
    expect(resolveSlots(times, 23 * 60)).toEqual({weekday: 23 * 60, saturday: 22 * 60 + 40})
    expect(pickSlot([600, 700], 721)).toBeNull()
    expect(pickSlot([], null)).toBeNull()
  })
})

describe('Cluj days', () => {
  it('knows the weekday and day type', () => {
    // 2026-10-09 is a Friday.
    expect(isoWeekday('2026-10-09')).toBe(5)
    expect(isoWeekday('2026-10-11')).toBe(7)
    expect(dayTypeOf('2026-10-10')).toBe('saturday')
    expect(dayTypeOf('2026-10-11')).toBe('sunday')
    expect(dayTypeOf('2026-10-12')).toBe('weekday')
    expect(addDays('2026-10-31', 1)).toBe('2026-11-01')
    expect(addDays('2026-01-01', -1)).toBe('2025-12-31')
  })

  it('turns a Cluj day and minute into the right instant across DST', () => {
    expect(clujAt('2026-10-09', 23 * 60 + 42)).toBe(ms('2026-10-09T23:42:00+03:00'))
    expect(clujAt('2026-12-01', 8 * 60)).toBe(ms('2026-12-01T08:00:00+02:00'))
    // Clocks go back on 2026-10-25.
    expect(clujAt('2026-10-24', 24 * 60 + 30)).toBe(ms('2026-10-25T00:30:00+03:00'))
    expect(clujAt('2026-10-25', 12 * 60)).toBe(ms('2026-10-25T12:00:00+02:00'))
    expect(clujDate(ms('2026-10-09T23:30:00+03:00'))).toBe('2026-10-09')
    expect(clujDate(ms('2026-10-09T21:30:00Z'))).toBe('2026-10-10')
    expect(formatSlot(24 * 60 + 5)).toBe('00:05')
    expect(formatSlot(23 * 60 + 9)).toBe('23:09')
  })
})

describe('nextOccurrence', () => {
  const at = (iso: string) => ms(`${iso}:00+03:00`)

  it('skips days without the reminder or a bus', () => {
    const slots = {weekday: 23 * 60 + 42, saturday: 23 * 60}
    const every = [1, 2, 3, 4, 5, 6, 7]
    expect(nextOccurrence(slots, every, null, at('2026-10-09T20:00'))).toBe(at('2026-10-09T23:42'))
    expect(nextOccurrence(slots, every, null, at('2026-10-09T23:50'))).toBe(at('2026-10-10T23:00'))
    expect(nextOccurrence(slots, every, null, at('2026-10-10T23:30'))).toBe(at('2026-10-12T23:42'))
    expect(nextOccurrence(slots, [1], null, at('2026-10-09T20:00'))).toBe(at('2026-10-12T23:42'))
  })

  it('runs a late bus on its service day, and a one-off only once', () => {
    const slots = {weekday: 24 * 60 + 5}
    expect(nextOccurrence(slots, [5], null, at('2026-10-09T23:00'))).toBe(at('2026-10-10T00:05'))
    expect(nextOccurrence(slots, [5], null, at('2026-10-10T00:01'))).toBe(at('2026-10-10T00:05'))
    expect(nextOccurrence(slots, [], '2026-10-09', at('2026-10-09T08:00'))).toBe(at('2026-10-10T00:05'))
    expect(nextOccurrence(slots, [], '2026-10-09', at('2026-10-10T00:06'))).toBeNull()
  })

  it('finds nothing without slots or days', () => {
    expect(nextOccurrence({}, [1, 2, 3, 4, 5, 6, 7], null, at('2026-10-09T08:00'))).toBeNull()
    expect(nextOccurrence({weekday: 600}, [], null, at('2026-10-09T08:00'))).toBeNull()
  })
})

describe('nextDeparture', () => {
  const times = {weekday: [8 * 60, 23 * 60, 24 * 60 + 5], saturday: [10 * 60]}

  it('finds the next bus, a late one on its own service day', () => {
    // 2026-10-09 is a Friday.
    expect(nextDeparture(times, ms('2026-10-09T07:00:00+03:00'))).toEqual({date: '2026-10-09', minute: 8 * 60})
    expect(nextDeparture(times, ms('2026-10-09T23:30:00+03:00'))).toEqual({date: '2026-10-09', minute: 24 * 60 + 5})
    expect(nextDeparture(times, ms('2026-10-10T00:10:00+03:00'))).toEqual({date: '2026-10-10', minute: 10 * 60})
  })

  it('runs the holiday timetable on special days', () => {
    setScheduleDays({'2026-10-12': 'saturday'})
    try {
      expect(dayTypeOf('2026-10-12')).toBe('saturday')
      expect(nextDeparture(times, ms('2026-10-12T07:00:00+03:00'))).toEqual({date: '2026-10-12', minute: 10 * 60})
    } finally {
      setScheduleDays(undefined)
    }
  })

  it('is null without buses', () => {
    expect(nextDeparture({}, ms('2026-10-09T07:00:00+03:00'))).toBeNull()
  })
})
