import {afterAll, beforeAll, describe, expect, it} from 'vitest'
import {formatMinutesFromNow, getMinutesFromDate, getTimetableDayKey} from '@/utils/time.ts'
import {cityAt, cityClock, cityDate, cityDateTime} from '@/utils/cityClock.ts'

// Saturday 23:30 in New York is Sunday 06:30 in Romania.
const saturdayNightInNewYork = new Date('2026-10-18T03:30:00Z')

describe('the city clock on a phone abroad', () => {
  let zone: string | undefined
  beforeAll(() => {
    zone = process.env.TZ
    process.env.TZ = 'America/New_York'
  })
  afterAll(() => {
    process.env.TZ = zone
  })

  it('reads the timetable of the day it is in the city', () => {
    expect(new Date(saturdayNightInNewYork).getDay()).toBe(6)
    expect(getTimetableDayKey(saturdayNightInNewYork)).toBe('sunday')
  })

  it('counts minutes from the city\'s midnight', () => {
    expect(getMinutesFromDate(saturdayNightInNewYork)).toBe(6 * 60 + 30)
    expect(formatMinutesFromNow(90, saturdayNightInNewYork, 'now')).toBe('08:00')
  })

  it('writes and reads city times', () => {
    expect(cityDate(saturdayNightInNewYork)).toBe('2026-10-18')
    expect(cityClock(saturdayNightInNewYork)).toBe('06:30')
    expect(cityDateTime(saturdayNightInNewYork)).toBe('2026-10-18T06:30')
    expect(cityAt('2026-10-18', 6 * 60 + 30)).toBe(saturdayNightInNewYork.getTime())
    expect(cityAt('2026-10-25', 2 * 60 + 30)).toBe(new Date('2026-10-24T23:30:00Z').getTime())
  })
})
