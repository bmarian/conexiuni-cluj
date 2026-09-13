import {describe, expect, it} from 'vitest'
import {mergeArrivals, minutesUntil, scheduledArrivals} from '@/utils/arrivals.ts'

// Route 45 towards Bulgaria, 14:42, captured from the stop list that showed the bug.
// Departures every 25 minutes; the bus on the road is running ~3 minutes ahead of its
// timetable and sits between Trifoiului Sud and Pinului.
const NOW = 14 * 60 + 42
const HEADWAY = 25
const departures = Array.from({length: 12}, (_, i) => 12 * 60 + 2 + i * HEADWAY)

// Minutes from the terminus to each stop, in travel order.
const offsets = {
  observatorului: 154,
  vitacom: 159,
  trifoiului: 160,
  pinului: 162,
  alverna: 164,
  tribunal: 170,
}

const at = (offsetMinutes: number) =>
  scheduledArrivals({departureMinutes: departures, offsetMinutes, nowMinutes: NOW, horizonMinutes: 480})

const labels = (offsetMinutes: number, live: number | null, tracked: boolean) =>
  mergeArrivals(live, at(offsetMinutes), {tracked}).map((a) => `${a.minutes}${a.isLive ? 'L' : ''}`)

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

describe('a stop the bus has already passed', () => {
  // The defect: the timetable had this run reaching Trifoiului Sud at exactly 14:42,
  // so the modulo version listed it as "now" while the bus was two stops further on,
  // and everything to its right shifted a column.
  it('does not advertise a run that live tracking says has gone by', () => {
    expect(at(offsets.trifoiului)[0]).toBe(0)
    expect(labels(offsets.trifoiului, null, true)).toEqual(['25', '50', '75'])
  })

  it('keeps showing it when nothing is being tracked, because then zero is a guess', () => {
    expect(labels(offsets.trifoiului, null, false)).toEqual(['0', '25', '50'])
  })

  it('leaves the columns reading forwards down the list', () => {
    const column = [
      labels(offsets.observatorului, null, true)[0],
      labels(offsets.vitacom, null, true)[0],
      labels(offsets.trifoiului, null, true)[0],
      labels(offsets.pinului, 0, true)[0],
      labels(offsets.alverna, 2, true)[0],
      labels(offsets.tribunal, 7, true)[0],
    ]
    // Before the fix this read 19, 24, 0, 0L, 2L, 7L - two ghosts in the middle.
    expect(column).toEqual(['19', '24', '25', '0L', '2L', '7L'])
  })
})

describe('merging a live estimate with the timetable', () => {
  it('takes the slot of the run it is tracking instead of sitting on top of it', () => {
    // Pinului is scheduled at 2m and the tracked bus is there now: one bus, one entry.
    expect(at(offsets.pinului).slice(0, 3)).toEqual([2, 27, 52])
    expect(labels(offsets.pinului, 0, true)).toEqual(['0L', '27', '52'])
  })

  it('does not swallow the next run when the tracked one is badly late', () => {
    // Its own slot has already expired, so the timetable starts at the next departure.
    const scheduled = [20, 45, 70]
    expect(mergeArrivals(2, scheduled).map((a) => a.minutes)).toEqual([2, 20, 45])
  })

  it('drops a timetable slot the live estimate has overtaken', () => {
    // Schedule says 1m, the bus is actually 6m out: the 1m entry is the same bus.
    expect(mergeArrivals(6, [1, 26, 51]).map((a) => a.minutes)).toEqual([6, 26, 51])
  })

  it('serves the live estimate alone when the timetable has run out for the day', () => {
    expect(mergeArrivals(4, [])).toEqual([{minutes: 4, isLive: true}])
    expect(mergeArrivals(null, [], {tracked: true})).toEqual([])
  })
})

describe('a run that is running late', () => {
  it('stays listed instead of wrapping into tomorrow and skipping a headway', () => {
    const late = scheduledArrivals({
      departureMinutes: [NOW - 2, NOW + 23],
      offsetMinutes: 0,
      nowMinutes: NOW,
      horizonMinutes: 480,
    })
    expect(late).toEqual([-2, 23])
    expect(mergeArrivals(null, late).map((a) => a.minutes)).toEqual([0, 23])
  })

  it('is gone once it is later than the grace window', () => {
    expect(scheduledArrivals({
      departureMinutes: [NOW - 4],
      offsetMinutes: 0,
      nowMinutes: NOW,
    })).toEqual([])
  })
})
