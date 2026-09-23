import {describe, expect, it} from 'vitest'
import type {Shape, StopTime, Vehicle} from '@/types/tranzy.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {
  buildShapeIndex,
  buildStopShapeIdxByStopId,
  etaForStop,
  getIndexedVehicles,
} from '@/composables/useVehicleTracking.ts'
import fixture from './fixtures/route32-trip25_1.json'

// Real geometry and real learned segment times for bus 32 towards Disp. Alverna,
// captured from production. Trip 25_1 starts at its terminus (P-ța M. Viteazul Vest)
// and the arrival terminus of the opposite direction sits ~65 m away, which is what
// makes this route the worst case for anything that snaps vehicles onto a shape.
const shape = fixture.shape as Shape[]
const stops = fixture.stops as StopTime[]
const index = buildShapeIndex(shape)
const stopShapeIdx = buildStopShapeIdxByStopId(stops, shape)

const FIXED_AT = new Date('2026-09-08T16:20:00.000Z')
const MAX_SPEED_MPS = 50 / 3.6

function vehicleAt(
  latitude: number,
  longitude: number,
  overrides: Partial<Vehicle> & { id?: number } = {},
): Vehicle {
  return {
    id: 1, label: '949', latitude, longitude,
    timestamp: FIXED_AT.toISOString(), vehicle_type: 3,
    bike_accessible: 'UNKNOWN', wheelchair_accessible: 'UNKNOWN',
    speed: 17, raw_speed: 27, stationary_since: FIXED_AT.toISOString(),
    route_id: 25, trip_id: '25_1',
    ...overrides,
  }
}

const track = (vehicles: Vehicle[], now: Date = FIXED_AT) =>
  getIndexedVehicles('25_1', '32', '#1e88e5', index, now, vehicles)

const etaAt = (stop: StopTime, vehicles: Awaited<ReturnType<typeof track>>, now: Date = FIXED_AT) =>
  etaForStop(stopShapeIdx.get(stop.stop_id) ?? -1, vehicles, index,
    {tripStops: stops, targetStopId: stop.stop_id, referenceTime: now})

describe('which vehicles are allowed to drive a live ETA', () => {
  it('draws a bus waiting at the departure terminus but leaves its stops on the timetable', async () => {
    // Parked at P-ța M. Viteazul Vest with a real speed of 0. The backend floors
    // Speed at 7 km/h for the ETA maths, so speed alone cannot tell this apart from
    // a bus crawling in traffic - stationary_since is what does.
    const parked = vehicleAt(46.77440, 23.59026, {id: 414, speed: 7, raw_speed: 0})
    const tracked = await track([parked])

    expect(tracked).toHaveLength(1)
    expect(tracked[0]!.atStartTerminus).toBe(true)
    for (const stop of stops) expect(etaAt(stop, tracked)).toBeNull()
  })

  it('ignores a bus that carries the trip but is nowhere near the route', async () => {
    // Tranzy assigns the next trip while a bus is still deadheading back. This one
    // was 635 m away on Horea and snapped to shape index 0, which made it the
    // nearest-behind candidate for every stop on the line.
    const deadheading = vehicleAt(46.77212, 23.58255, {id: 363, speed: 17.6, raw_speed: 8})
    const tracked = await track([deadheading])

    expect(tracked).toHaveLength(1) // still drawn on the map
    expect(tracked[0]!.shapeIdx).toBe(0)
    expect(tracked[0]!.offRouteMeters).toBeGreaterThan(600)
    for (const stop of stops) expect(etaAt(stop, tracked)).toBeNull()
  })

  it('uses a bus that is genuinely under way', async () => {
    const underway = vehicleAt(46.76280, 23.60960, {id: 454})
    const tracked = await track([underway])
    const ahead = stops.filter((s) => (stopShapeIdx.get(s.stop_id) ?? -1) > tracked[0]!.shapeIdx)

    expect(ahead.length).toBeGreaterThan(0)
    for (const stop of ahead) expect(etaAt(stop, tracked)).not.toBeNull()
  })
})

describe('an estimate credits the age of the fix, then waits for the next one', () => {
  // Tranzy refreshes a fix every ~30s while the backend polls faster, and a vehicle
  // it drops entirely is re-served from its last position for another 90s. The bus
  // has covered ground in that time, so the estimate may be credited for it - but
  // only up to a point, or a gap in the feed walks every stop down to "now" while
  // the bus stands hundreds of metres away.
  it('stops crediting once the fix is older than the cap', async () => {
    const bus = vehicleAt(46.76280, 23.60960, {id: 454})
    const fresh = await track([bus])
    const at = async (ageSeconds: number) => {
      const now = new Date(FIXED_AT.getTime() + ageSeconds * 1000)
      const tracked = await track([bus], now)
      return stops.map((s) => etaAt(s, tracked, now)?.etaSeconds ?? null)
    }

    const reference = stops.map((s) => etaAt(s, fresh)?.etaSeconds ?? null)
    const capped = await at(90)
    for (const ageSeconds of [30, 60, 90, 120, 150, 170]) {
      const seconds = await at(ageSeconds)
      seconds.forEach((eta, i) => {
        const before = reference[i]
        if (eta === null || before === null) return
        expect(before - eta).toBeLessThanOrEqual(90 + 0.5)
        expect(eta).toBeLessThanOrEqual(before + 0.5)
        // Past the cap the number holds until the bus reports again.
        if (ageSeconds >= 90) expect(eta).toBeCloseTo(capped[i]!, 5)
      })
    }
  })

  it('drops the live estimate once the fix is too old to mean anything', async () => {
    const bus = vehicleAt(46.76280, 23.60960, {id: 454})
    const now = new Date(FIXED_AT.getTime() + 5 * 60_000)
    const tracked = await track([bus], now)

    for (const stop of stops) expect(etaAt(stop, tracked, now)).toBeNull()
  })
})

describe('physical sanity along the whole route', () => {
  // Walk a bus over every point of the real shape at four speeds and assert the
  // estimates stay possible. 1800 estimates, no hand-picked positions.
  it('never claims a bus arrives sooner than it physically can', async () => {
    const impossible: string[] = []

    for (let idx = 0; idx < shape.length; idx++) {
      for (const speed of [7, 12, 20, 35]) {
        const point = shape[idx]!
        const tracked = await track([vehicleAt(point.shape_pt_lat, point.shape_pt_lon, {speed})])
        let previous = -1

        for (const stop of stops) {
          const eta = etaAt(stop, tracked)
          if (!eta) continue
          const metres = haversineMeters(point.shape_pt_lat, point.shape_pt_lon, stop.stop_lat, stop.stop_lon)
          const where = `idx=${idx} speed=${speed} ${stop.stop_headsign}`

          // Minutes are rounded for display, so "now" covers the last 30 seconds.
          if (eta.etaMinutes * 60 + 30 < metres / MAX_SPEED_MPS) {
            impossible.push(`${where}: ${eta.etaMinutes}m for ${metres.toFixed(0)}m`)
          }
          if (eta.etaMinutes === 0 && metres > 420) {
            impossible.push(`${where}: "now" at ${metres.toFixed(0)}m`)
          }
          // Stops are listed in travel order, so estimates may not go backwards.
          if (eta.etaMinutes < previous) {
            impossible.push(`${where}: ${eta.etaMinutes}m after ${previous}m at the previous stop`)
          }
          previous = eta.etaMinutes
        }
      }
    }

    expect(impossible).toEqual([])
  })

  // Asserted on raw seconds rather than displayed minutes: whole-minute rounding
  // flutters by design, and hiding a real discontinuity behind that tolerance is how
  // a one-minute step at every stop boundary went unnoticed.
  it('never makes a stop look further away as the bus approaches it', async () => {
    const regressions: string[] = []

    for (const speed of [7, 12, 20, 35]) {
      const readings = await Promise.all(shape.map(async (point) => {
        const tracked = await track([vehicleAt(point.shape_pt_lat, point.shape_pt_lon, {speed})])
        return stops.map((s) => etaAt(s, tracked)?.etaSeconds ?? null)
      }))

      stops.forEach((stop, i) => {
        let previous = Infinity
        readings.forEach((row, idx) => {
          const eta = row[i]
          if (eta === null) return
          if (eta > previous + 1) {
            regressions.push(`${stop.stop_headsign} speed=${speed}: ${previous.toFixed(0)}s -> ${eta.toFixed(0)}s at idx=${idx}`)
          }
          previous = Math.min(previous, eta)
        })
      })
    }

    expect(regressions).toEqual([])
  })
})

describe('a bus pulling out of the departure terminus', () => {
  // Bus 690 on line 9 left Disp. IRA at 10:04 on 2026-09-23, four minutes late. Still
  // inside the terminus radius, it was held to the next departure and the stop list
  // said 7 min for a stop it reached in one. A straight route stands in for the real
  // one: points every 20 m, the first stop after the terminus 120 m in.
  const STEP_DEG = 20 / 111_320
  const line = Array.from({length: 40}, (_, i): Shape => ({
    shape_id: '99_0', shape_pt_lat: 46.77 + i * STEP_DEG, shape_pt_lon: 23.6,
    shape_pt_sequence: i, shape_dist_traveled: -1,
  }))
  const lineIndex = buildShapeIndex(line)
  const lineStops = [0, 6, 30].map((pt, i): StopTime => ({
    trip_id: '99_0', stop_id: 900 + i, stop_sequence: i, offset_arrival_time: [0, 30, 120][i]!,
    stop_headsign: '', route_short_name: '9', stop_lat: line[pt]!.shape_pt_lat, stop_lon: line[pt]!.shape_pt_lon,
  }))
  const farStop = lineStops[2]!

  const etaFrom = async (pt: number, tripStops: StopTime[] = lineStops) => {
    const bus = vehicleAt(line[pt]!.shape_pt_lat, line[pt]!.shape_pt_lon, {id: 690, route_id: 99, trip_id: '99_0'})
    const tracked = await getIndexedVehicles('99_0', '9', '#000', lineIndex, FIXED_AT, [bus])
    expect(tracked[0]!.atStartTerminus).toBe(true)
    return etaForStop(30, tracked, lineIndex, {tripStops, targetStopId: farStop.stop_id, referenceTime: FIXED_AT})
  }

  it('drives a live estimate once it is past the first stop, radius or not', async () => {
    expect(await etaFrom(8)).not.toBeNull()
  })

  it('stays on the timetable while still short of that stop', async () => {
    expect(await etaFrom(4)).toBeNull()
  })

  it('stays on the timetable when the stops are not known', async () => {
    expect(await etaFrom(8, [])).toBeNull()
  })
})
