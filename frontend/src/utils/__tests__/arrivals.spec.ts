import {describe, expect, it} from 'vitest'
import {markRepeatedNow, minutesUntil, nextArrivals, scheduledArrivals} from '@/utils/arrivals.ts'

const NOW = 14 * 60 + 42

describe('minutesUntil', () => {
  it('reports a run that has already gone as negative rather than as almost a day away', () => {
    expect(minutesUntil(NOW - 1, NOW)).toBe(-1)
    expect(minutesUntil(NOW, NOW)).toBe(0)
    expect(minutesUntil(NOW + 18, NOW)).toBe(18)
  })

  it('still reads across midnight the short way round', () => {
    expect(minutesUntil(5, 23 * 60 + 50)).toBe(15)   // 00:05 seen from 23:50
    expect(minutesUntil(23 * 60 + 50, 5)).toBe(-15)  // 23:50 seen from 00:05
  })
})

describe('a run that is running late', () => {
  it('stays listed instead of wrapping into tomorrow and skipping a headway', () => {
    const late = scheduledArrivals({
      departureMinutes: [NOW - 2, NOW + 23],
      offsetSeconds: 0,
      nowMinutes: NOW,
      horizonMinutes: 480,
      graceMinutes: 3,
    })
    expect(late).toEqual([-2, 23])
    expect(nextArrivals(late, 3).map((a) => a.minutes)).toEqual([0, 23])
  })

  it('is gone once it is later than the grace the server learned', () => {
    expect(scheduledArrivals({departureMinutes: [NOW - 4], offsetSeconds: 0, nowMinutes: NOW, graceMinutes: 3})).toEqual([])
    expect(scheduledArrivals({departureMinutes: [NOW - 4], offsetSeconds: 0, nowMinutes: NOW, graceMinutes: 5})).toEqual([-4])
  })
})

describe('a stop list with nothing tracked', () => {
  // Bus 32 towards P-ța Mihai Viteazul at 09:17 on 2026-09-23, with none of its buses reported.
  const departures32 = [9 * 60 + 3, 9 * 60 + 12, 9 * 60 + 22, 9 * 60 + 31]
  const offsets32 = [0, 1, 2, 4, 7, 9, 14]
  const firstColumn = markRepeatedNow(offsets32.map((offsetMinutes) => nextArrivals(
    scheduledArrivals({departureMinutes: departures32, offsetSeconds: offsetMinutes * 60, nowMinutes: 9 * 60 + 17, graceMinutes: 3}),
    3,
  ))).map(([first]) => (first!.soon ? 'soon' : String(first!.minutes)))

  it('says "now" at the first of consecutive stops and "soon" at the rest', () => {
    expect(firstColumn).toEqual(['5', '6', '0', 'soon', '2', '4', '0'])
  })

  it('leaves a second "now" further down alone, since that is another bus', () => {
    expect(firstColumn.at(-1)).toBe('0')
  })

  it('reads a second zero within one row as "soon"', () => {
    const [row] = markRepeatedNow([[{minutes: 0, isLive: true}, {minutes: 0, isLive: false}, {minutes: 9, isLive: false}]])
    expect(row!.map((a) => a.soon)).toEqual([false, true, false])
  })
})
