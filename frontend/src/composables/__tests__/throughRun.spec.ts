import {describe, expect, it} from 'vitest'
import type {Shape, StopTime, Vehicle} from '@/types/tranzy.ts'
import {
  buildShapeIndex,
  getIndexedVehicles,
  liveEtasForStop,
  projectDeparture,
  type ThroughRunOptions,
} from '@/composables/useVehicleTracking.ts'
import {mergeArrivals} from '@/utils/arrivals.ts'

// Route 7: direction 0 runs north along a 1.1 km line, direction 1 runs it back and ends
// where direction 0 starts. 12:00, departures of direction 0 every 15 minutes.
const NOW = new Date('2026-10-12T09:00:00Z') // 12:00 in Cluj
const NOW_MIN = 12 * 60
const STEP = 0.0001

const line = (reverse: boolean): Shape[] =>
  Array.from({length: 100}, (_, i) => ({
    shape_id: reverse ? 'r' : 'f',
    shape_pt_lat: 46.77 + (reverse ? 99 - i : i) * STEP,
    shape_pt_lon: 23.6,
    shape_pt_sequence: i,
    shape_dist_traveled: 0,
  }))

const forward = line(false)
const backward = line(true)
const forwardIndex = buildShapeIndex(forward)
const backwardIndex = buildShapeIndex(backward)

const stop = (tripId: string, stopId: number, seq: number, shape: Shape[], idx: number, offset: number, extra: Partial<StopTime> = {}): StopTime => ({
  trip_id: tripId,
  stop_id: stopId,
  stop_sequence: seq,
  offset_arrival_time: offset,
  stop_headsign: `S${stopId}`,
  route_short_name: '7',
  stop_lat: shape[idx]!.shape_pt_lat,
  stop_lon: shape[idx]!.shape_pt_lon,
  ...extra,
})

const dir0Stops: StopTime[] = [
  stop('7_0', 1, 0, forward, 0, 45, {through_running: true, turnaround_sec: 90}),
  stop('7_0', 2, 1, forward, 40, 120),
  stop('7_0', 3, 2, forward, 90, 180),
]
const dir1Stops: StopTime[] = [
  stop('7_1', 4, 0, backward, 0, 0),
  stop('7_1', 5, 1, backward, 50, 100),
  stop('7_1', 6, 2, backward, 99, 100),
]
const stopIdx = {s1: 0, s2: 40, s3: 90}

const vehicle = (id: number, tripId: string, shape: Shape[], idx: number, extra: Partial<Vehicle> = {}): Vehicle => ({
  id,
  label: String(id),
  latitude: shape[idx]!.shape_pt_lat,
  longitude: shape[idx]!.shape_pt_lon,
  timestamp: new Date(NOW.getTime() - 10_000).toISOString(),
  vehicle_type: 3,
  bike_accessible: 'UNKNOWN',
  wheelchair_accessible: 'UNKNOWN',
  speed: 20,
  route_id: 7,
  trip_id: tripId,
  ...extra,
})

const indexed = (tripId: string, index: ReturnType<typeof buildShapeIndex>, vehicles: Vehicle[]) =>
  getIndexedVehicles(tripId, '7', '#000', index, NOW, vehicles)

const through = (departureMinutes: number[], feederVehicles: Vehicle[] = []): Promise<ThroughRunOptions> =>
  indexed('7_1', backwardIndex, feederVehicles).then((vehicles) => ({
    turnaround: {through: true, sec: 90},
    departureMinutes,
    nowMinutes: NOW_MIN,
    feeder: {vehicles, index: backwardIndex, tripStops: dir1Stops},
  }))

const etasAt = async (stopId: number, shapeIdx: number, own: Vehicle[], options: ThroughRunOptions | undefined) =>
  liveEtasForStop(shapeIdx, await indexed('7_0', forwardIndex, own), forwardIndex, {
    tripStops: dir0Stops,
    targetStopId: stopId,
    referenceTime: NOW,
    throughRun: options,
  }).map((eta) => Math.round(eta.etaSeconds))

const arrivedAgo = (seconds: number) => ({
  prev_trip_id: '7_1',
  trip_changed_at: new Date(NOW.getTime() - seconds * 1000).toISOString(),
})

describe('projectDeparture', () => {
  const options = (departureMinutes: number[]): ThroughRunOptions => ({
    turnaround: {through: true, sec: 90},
    departureMinutes,
    nowMinutes: NOW_MIN,
  })
  const now = NOW.getTime()

  it('leaves after the turnaround when the slot has already passed', () => {
    expect(projectDeparture(now - 30_000, options([705, 720, 735]), now, false)).toBe(now + 60_000)
  })

  it('waits for a slot that comes a little later', () => {
    expect(projectDeparture(now - 30_000, options([725, 740]), now, false)).toBe(now + 5 * 60_000)
  })

  it('does not wait for a slot that is further off than an early bus would', () => {
    expect(projectDeparture(now - 30_000, options([730, 745]), now, false)).toBe(now + 60_000)
  })

  it('moves a bus that has outstayed its departure onto the next slot', () => {
    expect(projectDeparture(now - 10 * 60_000, options([705, 720, 735]), now, true)).toBe(now)
    expect(projectDeparture(now - 10 * 60_000, options([705, 724]), now, true)).toBe(now + 4 * 60_000)
  })

  it('gives up on a standing bus with no slot in sight', () => {
    expect(projectDeparture(now - 10 * 60_000, options([705, 800]), now, true)).toBeNull()
  })

  it('reads a slot past midnight the short way round', () => {
    const late: ThroughRunOptions = {turnaround: {through: true, sec: 90}, departureMinutes: [2], nowMinutes: 23 * 60 + 58}
    expect(projectDeparture(now - 30_000, late, now, false)).toBe(now + 4 * 60_000)
  })
})

describe('a bus standing at a through terminus', () => {
  it('is about to leave, not parked', async () => {
    const own = [vehicle(1, '7_0', forward, 0, arrivedAgo(30))]
    expect(await etasAt(1, stopIdx.s1, own, await through([705, 720, 735]))).toEqual([60])
    expect(await etasAt(3, stopIdx.s3, own, await through([705, 720, 735]))).toEqual([360])
  })

  it('still reads as parked without the through flag', async () => {
    const own = [vehicle(1, '7_0', forward, 0, arrivedAgo(30))]
    const options = await through([705, 720, 735])
    options.turnaround = {through: false, sec: 90}
    expect(await etasAt(3, stopIdx.s3, own, options)).toEqual([])
    expect(await etasAt(3, stopIdx.s3, own, undefined)).toEqual([])
  })

  it('is ignored when it did not come off the other direction', async () => {
    const own = [vehicle(1, '7_0', forward, 0, {prev_trip_id: '8_1', trip_changed_at: new Date(NOW.getTime() - 30_000).toISOString()})]
    expect(await etasAt(3, stopIdx.s3, own, await through([705, 720, 735]))).toEqual([])
  })

  it('is forgotten once the flip is too old to be a turnaround', async () => {
    const own = [vehicle(1, '7_0', forward, 0, arrivedAgo(26 * 60))]
    expect(await etasAt(3, stopIdx.s3, own, await through([705, 720, 735]))).toEqual([])
  })
})

describe('a bus still finishing the other direction', () => {
  it('is listed on this direction after its ride in and the turnaround', async () => {
    const feeder = [vehicle(2, '7_1', backward, 50)]
    const options = await through([705, 720, 735], feeder)
    expect(await etasAt(1, stopIdx.s1, [], options)).toEqual([180])
    expect(await etasAt(3, stopIdx.s3, [], options)).toEqual([480])
  })

  it('comes after a bus already on this direction', async () => {
    const own = [vehicle(1, '7_0', forward, 50)]
    const feeder = [vehicle(2, '7_1', backward, 50)]
    const etas = await etasAt(3, stopIdx.s3, own, await through([705, 720, 735], feeder))
    expect(etas).toHaveLength(2)
    expect(etas[0]).toBeLessThan(etas[1]!)
    expect(etas[1]).toBe(480)
  })

  it('is left to the timetable while its departure is more than twelve minutes off', async () => {
    const feeder = [vehicle(2, '7_1', backward, 50)]
    const options = await through([705, 733], feeder)
    options.feederDepartureMinutes = [700, 715, 730]
    // Paired with 12:33, ten minutes after it is ready: within the paired cap, beyond the horizon.
    expect(await etasAt(1, stopIdx.s1, [], options)).toEqual([])
    options.departureMinutes = [705, 732]
    expect(await etasAt(1, stopIdx.s1, [], options)).toEqual([720])
  })

  it('is not listed while it waits to leave its own terminus', async () => {
    const feeder = [vehicle(2, '7_1', backward, 0)]
    expect(await etasAt(3, stopIdx.s3, [], await through([705, 720, 735], feeder))).toEqual([])
  })
})

describe('a bus tied to its timetable run', () => {
  // Direction 1 leaves every 15 minutes and takes 200 s; the feeder bus at mid-line left
  // Disp. at 12:03 and gets in at 12:01:30 + 90 s, so it is paired with the slot that
  // follows its scheduled arrival at 12:00:20.
  const withFeederSchedule = async (own: number[], feederDepartures: number[]) => {
    const options = await through(own, [vehicle(2, '7_1', backward, 50)])
    options.feederDepartureMinutes = feederDepartures
    return options
  }

  it('waits for its paired slot beyond the nearest-slot limit', async () => {
    expect(await etasAt(1, stopIdx.s1, [], await withFeederSchedule([705, 732], [700, 715, 730]))).toEqual([720])
    // A slot twenty minutes out is a timetable the bus is not keeping.
    expect(await etasAt(1, stopIdx.s1, [], await withFeederSchedule([705, 743], [700, 715, 730]))).toEqual([180])
    // Nothing pairs it without the feeder timetable, and the nearest slot is too far to wait for.
    expect(await etasAt(1, stopIdx.s1, [], await through([705, 732], [vehicle(2, '7_1', backward, 50)]))).toEqual([180])
  })

  it('leaves at once when its paired slot has passed', async () => {
    expect(await etasAt(1, stopIdx.s1, [], await withFeederSchedule([705, 718, 731], [700, 715, 730]))).toEqual([180])
  })

  it('falls back to the nearest slot when it sits between two runs', async () => {
    expect(await etasAt(1, stopIdx.s1, [], await withFeederSchedule([705, 725, 745], [702, 722, 742]))).toEqual([300])
  })
})

describe('merging several live estimates', () => {
  it('lists each tracked bus and only the timetable runs after them', () => {
    expect(mergeArrivals([18, 3], [4, 16, 28, 40]).map((a) => `${a.minutes}${a.isLive ? 'L' : ''}`)).toEqual(['3L', '18L', '28'])
  })

  it('treats an empty list like no estimate at all', () => {
    expect(mergeArrivals([], [0, 12], {tracked: true}).map((a) => a.minutes)).toEqual([12])
    expect(mergeArrivals([], [0, 12]).map((a) => a.minutes)).toEqual([0, 12])
  })
})
