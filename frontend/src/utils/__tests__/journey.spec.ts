import {describe, expect, it} from 'vitest'
import {
  ARRIVED_M, journeyParts, startRun, stepJourney, type Fix, type Journey, type JourneyRun, type LegDeparture,
} from '@/utils/journey.ts'

// Stops about 400 m apart going north from (46.760, 23.590).
const step = 0.0036
const stop = (id: number, i: number, name = `S${id}`) => ({id, name, lat: 46.76 + i * step, lon: 23.59})
const fixAt = (lat: number, lon: number, speed: number | null = null, accuracy = 10): Fix => ({lat, lon, accuracy, speed})
const at = (s: {lat: number, lon: number}, speed: number | null = null) => fixAt(s.lat, s.lon, speed)
const T0 = Date.parse('2026-10-09T18:00:00+03:00')

const leg1Stops = [stop(1, 0), stop(2, 1), stop(3, 2), stop(4, 3)]
const leg2Stops = [stop(5, 5), stop(6, 6), stop(7, 7)]
const destination = {id: -1, name: 'Destination', lat: 46.76 + 7.5 * step, lon: 23.5905}

function journey(overrides: Partial<Journey> = {}): Journey {
  return {
    id: 'j1',
    destination,
    legs: [
      {routeIds: [25], tripIds: ['25_0'], lines: ['25'], colors: ['#ff0000'], stops: leg1Stops, walkMinutes: 5, rideMinutes: 6},
      {routeIds: [35], tripIds: ['35_1'], lines: ['35'], colors: ['#0000ff'], stops: leg2Stops, walkMinutes: 4, rideMinutes: 4},
    ],
    walkEndMinutes: 3,
    walks: [],
    startedAt: T0,
    ...overrides,
  }
}

const bus = (minutes: number, line = '25'): LegDeparture => ({minutes, live: true, line, color: '#ff0000'})

function drive(j: Journey, run: JourneyRun, ticks: [Fix | null, LegDeparture | null, boolean?][]) {
  return ticks.map(([fix, dep, skip], i) => stepJourney(j, run, fix, dep, T0 + i * 30_000, skip ?? false))
}

describe('a planned trip', () => {
  it('starts at the stop when there is no walk to it', () => {
    const j = journey()
    j.legs[0]!.walkMinutes = 0
    expect(startRun(j, null).phase).toEqual({kind: 'wait', leg: 0})
    expect(startRun(journey(), null).phase).toEqual({kind: 'walk', leg: 0})
  })

  it('goes walk, wait, ride, walk, wait, ride, last walk, arrived as you move', () => {
    const j = journey()
    const start = fixAt(46.76 - 0.004, 23.59)
    const run = startRun(j, start)
    const results = drive(j, run, [
      [start, bus(6)],
      [fixAt(46.76 - 0.002, 23.59), bus(5)],
      [at(leg1Stops[0]!), bus(3)],
      [at(leg1Stops[0]!), bus(1)],
      [at(leg1Stops[1]!, 8), bus(0)],
      [at(leg1Stops[2]!, 8), null],
      [at(leg1Stops[3]!, 8), null],
      [fixAt(46.76 + 4.5 * step, 23.59), bus(7, '35')],
      [at(leg2Stops[0]!), bus(2, '35')],
      [at(leg2Stops[1]!, 7), null],
      [at(leg2Stops[2]!, 7), null],
      [fixAt(destination.lat, destination.lon), null],
    ])
    expect(results.map((r) => r.step.kind)).toEqual([
      'walk', 'walk', 'wait', 'wait', 'ride', 'ride', 'walk', 'walk', 'wait', 'ride', 'walkEnd', 'arrived',
    ])
    expect(results.at(-1)!.done).toBe(true)
  })

  it('says where to go and how long the walk is', () => {
    const j = journey()
    const start = fixAt(46.76 - 0.004, 23.59)
    const run = startRun(j, start)
    const [first] = drive(j, run, [[start, bus(6)]])
    expect(first!.step.target.name).toBe('S1')
    expect(first!.step.line).toBe('25')
    expect(first!.step.busMinutes).toBe(6)
    expect(first!.step.walkMinutes).toBe(6)
  })

  it('alerts once when the bus is a minute away, and once before the last stop', () => {
    const j = journey()
    const run = startRun(j, at(leg1Stops[0]!))
    const results = drive(j, run, [
      [at(leg1Stops[0]!), bus(3)],
      [at(leg1Stops[0]!), bus(1)],
      [at(leg1Stops[0]!), bus(1)],
      [at(leg1Stops[1]!, 8), bus(0)],
      [at(leg1Stops[2]!, 8), null],
      [at(leg1Stops[2]!, 8), null],
    ])
    expect(results.map((r) => r.alerted)).toEqual([false, true, false, false, true, false])
    expect(results[4]!.step.stopsLeft).toBe(1)
  })

  it('needs two moving fixes to believe you got on while still walking', () => {
    const j = journey()
    const run = startRun(j, fixAt(46.75, 23.59))
    const results = drive(j, run, [
      [at(leg1Stops[1]!, 8), bus(0)],
      [at(leg1Stops[2]!, 8), null],
    ])
    expect(results.map((r) => r.step.kind)).toEqual(['walk', 'ride'])
  })

  it('does not take a slow walk past the stop for a ride', () => {
    const j = journey()
    const run = startRun(j, fixAt(46.75, 23.59))
    const results = drive(j, run, [
      [at(leg1Stops[1]!, 1.2), bus(4)],
      [at(leg1Stops[2]!, 1.4), bus(4)],
    ])
    expect(results.map((r) => r.step.kind)).toEqual(['walk', 'walk'])
  })

  it('moves on without a location: the bus leaving, or the time it takes', () => {
    const j = journey()
    const run = startRun(j, null)
    const results = drive(j, run, [
      [null, bus(1)],
      [null, bus(0)],
      [null, bus(15)],
    ])
    expect(results.map((r) => r.step.kind)).toEqual(['walk', 'walk', 'ride'])
    const later = stepJourney(j, run, null, null, T0 + 60_000 + 7 * 60_000)
    expect(later.step.kind).toBe('walk')
    expect(later.step.legIndex).toBe(1)
  })

  it('does not reuse one leg’s departed bus to start the next ride', () => {
    const j = journey()
    const run = startRun(j, null)
    drive(j, run, [[null, bus(0)], [null, bus(12)]])
    expect(run.phase).toEqual({kind: 'ride', leg: 0})
    stepJourney(j, run, null, null, T0 + 120_000, true)
    expect(run.phase).toEqual({kind: 'walk', leg: 1})
    expect(stepJourney(j, run, null, bus(14, '35'), T0 + 150_000).step.kind).toBe('walk')
  })

  it('steps on when asked, all the way to the end', () => {
    const j = journey()
    const run = startRun(j, null)
    const kinds = []
    for (let i = 0; i < 7; i++) kinds.push(stepJourney(j, run, null, null, T0 + i * 1000, true).step.kind)
    expect(kinds).toEqual(['wait', 'ride', 'walk', 'wait', 'ride', 'walkEnd', 'arrived'])
  })

  it('arrives at a stop destination without a last walk', () => {
    const j = journey({destination: {...leg2Stops[2]!}})
    expect(journeyParts(j).at(-1)!.ride).toBe(true)
    const run = startRun(j, null)
    const kinds = []
    for (let i = 0; i < 6; i++) kinds.push(stepJourney(j, run, null, null, T0 + i * 1000, true).step.kind)
    expect(kinds).toEqual(['wait', 'ride', 'walk', 'wait', 'ride', 'arrived'])
  })

  it('goes straight to waiting when the next leg leaves from the same stop', () => {
    const shared = {...leg2Stops[0]!, lat: leg1Stops[3]!.lat, lon: leg1Stops[3]!.lon}
    const j = journey()
    j.legs[1]!.stops = [shared, ...leg2Stops.slice(1)]
    const run = startRun(j, null)
    stepJourney(j, run, null, null, T0, true)
    stepJourney(j, run, null, null, T0 + 1000, true)
    expect(stepJourney(j, run, null, null, T0 + 2000, true).step.kind).toBe('wait')
  })

  it('arrives within 50 m of the destination, or after twice the walk and ten minutes', () => {
    const j = journey()
    const run = startRun(j, null)
    for (let i = 0; i < 6; i++) stepJourney(j, run, null, null, T0 + i * 1000, true)
    expect(run.phase.kind).toBe('walkEnd')
    const near = fixAt(destination.lat + (ARRIVED_M - 10) / 111_000, destination.lon)
    expect(stepJourney(j, run, near, null, T0 + 10_000).done).toBe(true)

    const run2 = startRun(j, null)
    for (let i = 0; i < 6; i++) stepJourney(j, run2, null, null, T0 + i * 1000, true)
    expect(stepJourney(j, run2, null, null, T0 + 9 * 60_000).done).toBe(false)
    expect(stepJourney(j, run2, null, null, T0 + 11 * 60_000).done).toBe(true)
  })

  it('tracks progress and arrival time, waiting for a bus that comes late', () => {
    const j = journey()
    const run = startRun(j, at(leg1Stops[0]!))
    const total = journeyParts(j).reduce((a, p) => a + p.minutes, 0)
    expect(total).toBe(5 + 6 + 4 + 4 + 3)
    const waiting = stepJourney(j, run, at(leg1Stops[0]!), bus(20), T0)
    expect(waiting.step.kind).toBe('wait')
    expect(waiting.step.progress).toBeCloseTo(5 / total)
    expect(waiting.step.etaAt).toBe(T0 + (20 + 6 + 4 + 4 + 3) * 60_000)
  })
})

describe('the arrival estimate', () => {
  const s = (minutes: number) => Math.floor(T0 / 1000) + minutes * 60
  // The 18:06 bus makes the 18:20 change and arrives 18:40; the 18:16 one waits for the 18:35 and arrives 18:55.
  const runs = [{arrive: s(40), departs: [s(6), s(20)]}, {arrive: s(55), departs: [s(16), s(35)]}]

  it('takes the planned run of the bus coming, waits at the change included', () => {
    const j = journey({runs})
    const run = startRun(j, null)
    expect(stepJourney(j, run, null, bus(6), T0).step.etaAt).toBe(s(40) * 1000)
    expect(stepJourney(j, run, null, bus(16), T0).step.etaAt).toBe(s(55) * 1000)
  })

  it('is later by as much as the bus is', () => {
    const j = journey({runs})
    expect(stepJourney(j, startRun(j, null), null, bus(8), T0).step.etaAt).toBe(s(42) * 1000)
  })

  it('falls back to the timetable sum without planned runs', () => {
    const j = journey()
    const eta = stepJourney(j, startRun(j, null), null, bus(6), T0).step.etaAt
    expect(eta).toBeLessThan(s(40) * 1000)
  })
})
