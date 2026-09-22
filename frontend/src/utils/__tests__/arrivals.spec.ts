import {describe, expect, it} from 'vitest'
import type {Shape, StopTime, Vehicle} from '@/types/tranzy.ts'
import {arrivalsAlongTrip, minutesUntil, scheduledArrivals} from '@/utils/arrivals.ts'
import {buildShapeIndex, getIndexedVehicles, stopShapePositions} from '@/composables/useVehicleTracking.ts'
import {timeStringToMinutes} from '@/utils/time.ts'
import fixture from './fixtures/route32-trip25_0.json'

// Bus 32 towards P-ța Mihai Viteazul on 2026-09-22, from production: geometry, learned
// segment times, the weekday timetable and both buses on the road at 18:38.
const shape = fixture.shape as Shape[]
const stops = fixture.stops as StopTime[]
const index = buildShapeIndex(shape)
const positions = stopShapePositions(stops, shape)
const departures = fixture.weekdayDepartures.map((d) => timeStringToMinutes(d)!)
const REPORTED_AT = new Date(2026, 8, 22, 18, 38, 40)

type Fix = { id: number; label: string; latitude: number; longitude: number; speed: number; raw_speed: number }

function vehicle(fix: Fix, now: Date, stationary = false): Vehicle {
  const at = new Date(now.getTime() - 8_000).toISOString()
  return {
    ...fix, timestamp: at, vehicle_type: 3,
    bike_accessible: 'UNKNOWN', wheelchair_accessible: 'UNKNOWN',
    stationary_since: stationary ? new Date(now.getTime() - 5 * 60_000).toISOString() : at,
    route_id: 25, trip_id: '25_0',
  }
}

const atShapePoint = (idx: number, id = 1): Fix => ({
  id, label: String(id), latitude: shape[idx]!.shape_pt_lat, longitude: shape[idx]!.shape_pt_lon, speed: 18, raw_speed: 18,
})

async function board(fixes: Vehicle[], now = REPORTED_AT, deps = departures): Promise<string[][]> {
  const vehicles = await getIndexedVehicles('25_0', '32', '#2B73DE', index, now, fixes)
  return arrivalsAlongTrip({departureMinutes: deps, tripStops: stops, vehicles, index, referenceTime: now, horizonMinutes: 480})
    .map((list) => list.map((a) => `${a.minutes}${a.isLive ? 'L' : ''}`))
}

describe('minutesUntil', () => {
  it('reports a run that has already gone as negative rather than as almost a day away', () => {
    expect(minutesUntil(600 - 1, 600)).toBe(-1)
    expect(minutesUntil(600, 600)).toBe(0)
    expect(minutesUntil(600 + 18, 600)).toBe(18)
  })

  it('still reads across midnight the short way round', () => {
    expect(minutesUntil(5, 23 * 60 + 50)).toBe(15)
    expect(minutesUntil(23 * 60 + 50, 5)).toBe(-15)
  })
})

describe('scheduledArrivals', () => {
  it('keeps a run a couple of minutes late instead of wrapping it into tomorrow', () => {
    expect(scheduledArrivals({departureMinutes: [598, 623], offsetMinutes: 0, nowMinutes: 600})).toEqual([-2, 23])
  })

  it('drops it once it is later than the grace window', () => {
    expect(scheduledArrivals({departureMinutes: [596], offsetMinutes: 0, nowMinutes: 600})).toEqual([])
  })
})

describe('the route 32 stop list at 18:38', () => {
  it('lists each bus once, and only at stops it has not reached yet', async () => {
    const [bus949, bus950] = fixture.vehiclesAt183832
    // What it used to show: 11 24 41 / 5 18 31 / 7 20 33 / 1L 8 21 / 5L 12 25 / 6L 13 26 / 1L 6 19.
    // Bus 949 (the 18:36 run) was listed behind itself at Alverna Est and Mălinului Est,
    // and again from the timetable at every stop ahead of it.
    expect(await board([vehicle(bus949!, REPORTED_AT), vehicle(bus950!, REPORTED_AT)])).toEqual([
      ['11', '24', '41'],
      ['18', '31', '48'],
      ['20', '33', '50'],
      ['1L', '21', '34'],
      ['4L', '25', '38'],
      ['6L', '26', '39'],
      ['1L', '11L', '32'],
    ])
  })
})

describe('one bus driving the 18:36 run end to end', () => {
  const RUN = 18 * 60 + 36
  const NEXT = 18 * 60 + 49
  const cumulative: number[] = []
  stops.reduce((sum, st) => {
    cumulative.push(sum + st.offset_arrival_time)
    return sum + st.offset_arrival_time
  }, 0)
  const offsetMin = cumulative.map((sec) => Math.ceil(sec / 60))

  // Seconds into the run a bus exactly on its timetable reaches this shape point.
  const onSchedule = (idx: number): number => {
    let i = 0
    while (i + 1 < positions.length && positions[i + 1]! <= idx) i++
    if (i + 1 >= positions.length) return cumulative[i]!
    const a = index.cumulativeDist[positions[i]!]!, b = index.cumulativeDist[positions[i + 1]!]!
    const share = b > a ? Math.max(0, (index.cumulativeDist[idx]! - a) / (b - a)) : 0
    return cumulative[i]! + share * (cumulative[i + 1]! - cumulative[i]!)
  }

  // Runs are 13 min apart here, so past ~6 min late a bus is indistinguishable from the next run early.
  for (const lateMin of [-2, 0, 3, 6]) {
    it(`never lists it twice or behind itself when ${lateMin} min off schedule`, async () => {
      const failures: string[] = []
      for (let idx = 8; idx < shape.length - 1; idx++) {
        const now = new Date(2026, 8, 22, 0, 0, 0)
        now.setTime(now.getTime() + (RUN * 60 + onSchedule(idx) + lateMin * 60) * 1000)
        const nowMin = now.getHours() * 60 + now.getMinutes()
        const rows = await board([vehicle(atShapePoint(idx), now)], now)
        rows.forEach((row, k) => {
          const thisRunHere = RUN + offsetMin[k]! - nowMin
          const live = row.filter((label) => label.endsWith('L'))
          const scheduled = row.filter((label) => !label.endsWith('L')).map(Number)
          const ahead = positions[k]! >= idx
          if (ahead && live.length !== 1) failures.push(`idx ${idx} stop ${k}: ${live.length} live in ${row}`)
          if (!ahead && live.length) failures.push(`idx ${idx} stop ${k}: live behind the bus in ${row}`)
          if (scheduled.some((m) => Math.abs(m - thisRunHere) < (NEXT - RUN) / 2)) {
            failures.push(`idx ${idx} stop ${k}: 18:36 run still scheduled in ${row}`)
          }
        })
      }
      expect(failures).toEqual([])
    })
  }
})

describe('buses at the departure terminus', () => {
  it('holds an overdue departure at "now" while its bus is still standing there', async () => {
    const [, bus950] = fixture.vehiclesAt183832
    const waiting = vehicle(atShapePoint(0, 7), REPORTED_AT, true)
    const elsewhere = vehicle(bus950!, REPORTED_AT)

    const withBus = await board([waiting, elsewhere])
    expect(withBus[0]).toEqual(['0', '11', '24'])
    expect(withBus[1]).toEqual(['7', '18', '31'])

    // Tracking is live and nothing is waiting, so the 18:36 departure has gone.
    const withoutBus = await board([elsewhere])
    expect(withoutBus[0]).toEqual(['11', '24', '41'])
  })
})

describe('without a usable timetable or tracking', () => {
  it('still shows live buses on a headway-only direction', async () => {
    const [bus949] = fixture.vehiclesAt183832
    const rows = await board([vehicle(bus949!, REPORTED_AT)], REPORTED_AT, [])
    expect(rows.slice(0, 3)).toEqual([[], [], []])
    expect(rows.slice(3).every((row) => row.length === 1 && row[0]!.endsWith('L'))).toBe(true)
  })

  it('falls back to the timetable, keeping a run a couple of minutes late', async () => {
    expect((await board([]))[0]).toEqual(['0', '11', '24'])
  })
})
