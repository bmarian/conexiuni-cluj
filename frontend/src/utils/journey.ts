import {haversineMeters} from '@/utils/geo.ts'
import {WALKING_SPEED_M_PER_MIN} from '@/utils/trips.ts'
import {alongEach, RouteLine} from '@/utils/rideLine.ts'

export const AT_STOP_M = 40
export const ARRIVED_M = 50
const LEFT_STOP_M = 150
const ON_LINE_M = 300
const RIDE_LINE_M = 75
// 72 km/h, faster than a bus goes in town.
const MAX_BUS_MPS = 20
// ≈16 km/h
const BUS_SPEED_MPS = 4.5
const RIDING_FIXES = 2
export const JOURNEY_GIVE_UP_MS = 4 * 60 * 60_000
export const NAV_POSITION_OPTIONS: PositionOptions = {enableHighAccuracy: true, maximumAge: 2_000, timeout: 20_000}

export interface JourneyStop {
  id: number
  name: string
  lat: number
  lon: number
}

export interface JourneyLeg {
  routeIds: number[]
  tripIds: string[]
  lines: string[]
  colors: string[]
  stops: JourneyStop[]
  walkMinutes: number
  rideMinutes: number
  /** The ride's way along the line, [lat, lon]. */
  path?: [number, number][]
}

export interface Journey {
  id: string
  destination: JourneyStop
  legs: JourneyLeg[]
  walkEndMinutes: number
  /** [lat, lon] */
  walks: [number, number][][]
  planPath?: string
  /** The plan's timed runs: when each ride's bus leaves (unix s) and when the run arrives. */
  runs?: PlannedRun[]
  startedAt: number
}

export interface PlannedRun {
  arrive: number
  departs: number[]
}

export interface Fix {
  lat: number
  lon: number
  accuracy: number
  speed: number | null
}

export interface LegDeparture {
  minutes: number
  live: boolean
  line: string
  color: string
}

export type Phase =
  | {kind: 'walk', leg: number}
  | {kind: 'wait', leg: number}
  | {kind: 'ride', leg: number}
  | {kind: 'walkEnd'}
  | {kind: 'done'}

export interface JourneyRun {
  phase: Phase
  stepIndex: number
  alerted: boolean
  rideStop: number
  rideStartedAt: number
  lastDepartureMinutes: number | null
  walkEndStartedAt: number
  walkStartM: number
  ridingInARow: number
  awayInARow: number
  rideAlong: number
  rideFixAt: number
  atStopAt: number
  seenFix: string | null
}

export type StepKind = 'walk' | 'wait' | 'ride' | 'walkEnd' | 'arrived'

export interface JourneyStep {
  kind: StepKind
  legIndex: number
  target: JourneyStop
  line: string | null
  color: string | null
  busMinutes: number | null
  busLive: boolean
  walkMinutes: number | null
  stopsLeft: number | null
  totalStops: number | null
  alert: boolean
  progress: number
  etaAt: number
  stepIndex: number
}

export const lineLabel = (leg: JourneyLeg) => leg.lines.join(' / ')
export function endsAtStop(journey: Journey): boolean {
  const last = journey.legs[journey.legs.length - 1]
  return !!last && journey.destination.id === last.stops[last.stops.length - 1]?.id
}
const boarding = (leg: JourneyLeg) => leg.stops[0]!
const alighting = (leg: JourneyLeg) => leg.stops[leg.stops.length - 1]!
const legOf = (phase: Phase) => ('leg' in phase ? phase.leg : -1)

export const distanceTo = (fix: Fix, stop: JourneyStop) => haversineMeters(fix.lat, fix.lon, stop.lat, stop.lon)

export function nearestStop(fix: Fix, leg: JourneyLeg): number | null {
  let best = -1
  let bestM = Infinity
  leg.stops.forEach((s, i) => {
    const m = distanceTo(fix, s)
    if (m < bestM) {
      bestM = m
      best = i
    }
  })
  return best >= 0 && bestM <= ON_LINE_M ? best : null
}

export function leftOnBus(fix: Fix, leg: JourneyLeg): boolean {
  return distanceTo(fix, boarding(leg)) > LEFT_STOP_M && (nearestStop(fix, leg) ?? 0) >= 1
}

export function startRun(journey: Journey, fix: Fix | null): JourneyRun {
  const first = journey.legs[0]!
  return {
    phase: {kind: first.walkMinutes < 0.5 ? 'wait' : 'walk', leg: 0},
    stepIndex: 0,
    alerted: false,
    rideStop: 0,
    rideStartedAt: 0,
    lastDepartureMinutes: null,
    walkEndStartedAt: 0,
    walkStartM: fix ? distanceTo(fix, boarding(first)) : 0,
    ridingInARow: 0,
    awayInARow: 0,
    rideAlong: 0,
    rideFixAt: 0,
    atStopAt: 0,
    seenFix: null,
  }
}

// Ticks repeat the last fix, so counts in a row only take new ones.
const fixKey = (fix: Fix) => `${fix.lat},${fix.lon},${fix.accuracy}`

function onBus(run: JourneyRun, fix: Fix, leg: JourneyLeg, fresh: boolean): boolean {
  if (fresh) {
    const moving = fix.speed !== null && fix.speed >= BUS_SPEED_MPS && leftOnBus(fix, leg)
    run.ridingInARow = moving ? run.ridingInARow + 1 : 0
  }
  return run.ridingInARow >= RIDING_FIXES
}

const rideRoute = (leg: JourneyLeg) =>
  new RouteLine(leg.path && leg.path.length >= 2 ? leg.path : leg.stops.map((s): [number, number] => [s.lat, s.lon]))

// A fix only counts if it is near the line and the bus could have got there since the last one,
// so a stray fix near a later part of the line does not skip stops.
function trackRide(run: JourneyRun, route: RouteLine, stops: number[], fix: Fix, nowMs: number) {
  const error = fix.accuracy
  const reach = (MAX_BUS_MPS * Math.max(nowMs - run.rideFixAt, 0)) / 1000 + 2 * error + AT_STOP_M
  const spot = route.locate(fix.lat, fix.lon, run.rideAlong - error, run.rideAlong + reach)
  if (!spot || spot.off > Math.max(RIDE_LINE_M, 2 * error)) return
  run.rideAlong = Math.max(run.rideAlong, spot.along - error)
  run.rideFixAt = nowMs
  stops.forEach((along, i) => {
    if (along <= run.rideAlong) run.rideStop = Math.max(run.rideStop, i)
  })
}

function busLeft(run: JourneyRun, departure: LegDeparture | null): boolean {
  const minutes = departure?.minutes ?? null
  const left = run.lastDepartureMinutes === 0 && minutes !== null && minutes > 2
  run.lastDepartureMinutes = minutes
  return left
}

function within(fix: Fix, stop: JourneyStop, meters: number) {
  return distanceTo(fix, stop) <= Math.max(meters, fix.accuracy)
}

function advance(journey: Journey, run: JourneyRun, fix: Fix | null, departure: LegDeparture | null, nowMs: number, skip: boolean): Phase {
  const afterLeg = (i: number): Phase => {
    if (i + 1 < journey.legs.length) {
      const here = alighting(journey.legs[i]!)
      const next = boarding(journey.legs[i + 1]!)
      return haversineMeters(here.lat, here.lon, next.lat, next.lon) <= AT_STOP_M ? {kind: 'wait', leg: i + 1} : {kind: 'walk', leg: i + 1}
    }
    return endsAtStop(journey) ? {kind: 'done'} : {kind: 'walkEnd'}
  }
  const phase = run.phase
  const fresh = !!fix && fixKey(fix) !== run.seenFix
  switch (phase.kind) {
    case 'walk': {
      const leg = journey.legs[phase.leg]!
      if (skip) return {kind: 'wait', leg: phase.leg}
      if (fix && within(fix, boarding(leg), AT_STOP_M)) {
        run.atStopAt = nowMs
        return {kind: 'wait', leg: phase.leg}
      }
      if (fix && onBus(run, fix, leg, fresh)) return {kind: 'ride', leg: phase.leg}
      if (!fix && busLeft(run, departure)) return {kind: 'ride', leg: phase.leg}
      return phase
    }
    case 'wait': {
      const leg = journey.legs[phase.leg]!
      // One fix away from the stop can be the GPS wandering while you wait.
      if (fix && fresh) {
        run.awayInARow = leftOnBus(fix, leg) ? run.awayInARow + 1 : 0
        if (within(fix, boarding(leg), AT_STOP_M)) run.atStopAt = nowMs
      }
      if (skip) return {kind: 'ride', leg: phase.leg}
      if (fix && (run.awayInARow >= 2 || onBus(run, fix, leg, fresh))) return {kind: 'ride', leg: phase.leg}
      if (!fix && busLeft(run, departure)) return {kind: 'ride', leg: phase.leg}
      return phase
    }
    case 'ride': {
      const leg = journey.legs[phase.leg]!
      const last = leg.stops.length - 1
      const route = rideRoute(leg)
      const stops = alongEach(route, leg.stops)
      if (fix) {
        if (fresh) trackRide(run, route, stops, fix, nowMs)
      } else {
        const perStop = leg.rideMinutes > 0 ? leg.rideMinutes / Math.max(last, 1) : 2
        run.rideStop = Math.max(run.rideStop, Math.min(Math.floor((nowMs - run.rideStartedAt) / 60_000 / perStop), last))
      }
      const there = !!fix && run.rideAlong >= stops[last]! - AT_STOP_M
      return skip || there || run.rideStop >= last ? afterLeg(phase.leg) : phase
    }
    case 'walkEnd': {
      if (skip) return {kind: 'done'}
      if (fix && within(fix, journey.destination, ARRIVED_M)) return {kind: 'done'}
      const walkMin = fix ? distanceTo(fix, journey.destination) / WALKING_SPEED_M_PER_MIN : 0
      if (nowMs - run.walkEndStartedAt > (walkMin * 2 + 10) * 60_000) return {kind: 'done'}
      return phase
    }
    case 'done':
      return phase
  }
}

interface Part {leg: number, ride: boolean, minutes: number}

export function journeyParts(journey: Journey): Part[] {
  const parts: Part[] = []
  journey.legs.forEach((leg, i) => {
    if (leg.walkMinutes >= 0.5) parts.push({leg: i, ride: false, minutes: leg.walkMinutes})
    parts.push({leg: i, ride: true, minutes: leg.rideMinutes > 0 ? leg.rideMinutes : (leg.stops.length - 1) * 2})
  })
  if (!endsAtStop(journey)) parts.push({leg: -1, ride: false, minutes: Math.max(journey.walkEndMinutes, 1)})
  return parts
}

function doneMinutes(journey: Journey, run: JourneyRun, parts: Part[], fix: Fix | null): number {
  const p = run.phase
  const sum = (list: Part[]) => list.reduce((acc, part) => acc + part.minutes, 0)
  switch (p.kind) {
    case 'walk': {
      const walk = parts.findIndex((part) => part.leg === p.leg && !part.ride)
      const part = parts[walk]
      const fraction = fix && run.walkStartM > 0 ? 1 - distanceTo(fix, boarding(journey.legs[p.leg]!)) / run.walkStartM : 0
      return sum(parts.slice(0, Math.max(walk, 0))) + (part?.minutes ?? 0) * Math.min(Math.max(fraction, 0), 1)
    }
    case 'wait': {
      const ride = parts.findIndex((part) => part.leg === p.leg && part.ride)
      return sum(parts.slice(0, ride))
    }
    case 'ride': {
      const ride = parts.findIndex((part) => part.leg === p.leg && part.ride)
      const stops = Math.max(journey.legs[p.leg]!.stops.length - 1, 1)
      return sum(parts.slice(0, ride)) + parts[ride]!.minutes * run.rideStop / stops
    }
    case 'walkEnd':
      return sum(parts.slice(0, -1))
    case 'done':
      return sum(parts)
  }
}

// The planned run the traveller is on: its bus on this leg leaves nearest to the one they wait for or
// boarded, and a late bus makes it that much later. Null when there is no bus to match it by.
function plannedArrival(journey: Journey, run: JourneyRun, departure: LegDeparture | null, nowMs: number): number | null {
  const p = run.phase
  if (!journey.runs?.length || (p.kind !== 'walk' && p.kind !== 'wait' && p.kind !== 'ride')) return null
  const riding = p.kind === 'ride'
  const bus = riding ? run.rideStartedAt : departure ? nowMs + departure.minutes * 60_000 : null
  if (bus === null) return null
  let best: PlannedRun | null = null
  for (const planned of journey.runs) {
    const leaves = planned.departs[p.leg]
    if (leaves === undefined) continue
    if (!best || Math.abs(leaves * 1000 - bus) < Math.abs(best.departs[p.leg]! * 1000 - bus)) best = planned
  }
  if (!best) return null
  const late = riding ? 0 : Math.max(0, bus - best.departs[p.leg]! * 1000)
  return best.arrive * 1000 + late
}

function arrivalAt(run: JourneyRun, parts: Part[], done: number, departure: LegDeparture | null, nowMs: number): number {
  const total = parts.reduce((acc, part) => acc + part.minutes, 0)
  const left = Math.max(total - done, 0)
  const p = run.phase
  let minutes = left
  if ((p.kind === 'walk' || p.kind === 'wait') && departure) {
    const ride = Math.max(parts.findIndex((part) => part.leg === p.leg && part.ride), 0)
    const fromBoarding = parts.slice(ride).reduce((acc, part) => acc + part.minutes, 0)
    minutes = Math.max(departure.minutes, left - fromBoarding) + fromBoarding
  }
  return nowMs + minutes * 60_000
}

function describe(journey: Journey, run: JourneyRun, fix: Fix | null, departure: LegDeparture | null): Omit<JourneyStep, 'progress' | 'etaAt' | 'stepIndex'> {
  const p = run.phase
  const legIndex = Math.max(legOf(p), 0)
  const walkTo = (stop: JourneyStop) => fix ? Math.max(1, Math.round(distanceTo(fix, stop) / WALKING_SPEED_M_PER_MIN)) : null
  const base = {legIndex, busMinutes: null, busLive: false, walkMinutes: null, stopsLeft: null, totalStops: null, alert: false}
  switch (p.kind) {
    case 'walk':
    case 'wait': {
      const leg = journey.legs[p.leg]!
      return {
        ...base,
        kind: p.kind,
        target: boarding(leg),
        line: departure?.line ?? leg.lines[0] ?? null,
        color: departure?.color ?? leg.colors[0] ?? null,
        busMinutes: departure?.minutes ?? null,
        busLive: departure?.live ?? false,
        walkMinutes: p.kind === 'walk' ? walkTo(boarding(leg)) : null,
        alert: p.kind === 'wait' && departure !== null && departure.minutes <= 1,
      }
    }
    case 'ride': {
      const leg = journey.legs[p.leg]!
      const left = Math.max(leg.stops.length - 1 - run.rideStop, 1)
      return {
        ...base,
        kind: 'ride',
        target: alighting(leg),
        line: departure?.line ?? leg.lines[0] ?? null,
        color: departure?.color ?? leg.colors[0] ?? null,
        stopsLeft: left,
        totalStops: leg.stops.length - 1,
        alert: left === 1,
      }
    }
    case 'walkEnd':
      return {...base, kind: 'walkEnd', target: journey.destination, line: null, color: null, walkMinutes: walkTo(journey.destination)}
    case 'done':
      return {...base, kind: 'arrived', target: journey.destination, line: null, color: null}
  }
}

export interface StepResult {
  step: JourneyStep
  changed: boolean
  alerted: boolean
  done: boolean
}

export function stepJourney(journey: Journey, run: JourneyRun, fix: Fix | null, departure: LegDeparture | null, nowMs: number, skip = false): StepResult {
  const before = run.phase
  const next = advance(journey, run, fix, departure, nowMs, skip)
  const changed = JSON.stringify(next) !== JSON.stringify(before)
  if (changed) {
    run.phase = next
    run.stepIndex++
    run.alerted = false
    run.ridingInARow = 0
    run.awayInARow = 0
    if (next.kind === 'ride' && before.kind !== 'ride') {
      const leg = journey.legs[next.leg]!
      const route = rideRoute(leg)
      const stops = alongEach(route, leg.stops)
      run.rideStop = 0
      run.rideStartedAt = nowMs
      run.rideAlong = stops[0]!
      // From the last time you were seen at the stop: the bus may be well on its way by now.
      run.rideFixAt = run.atStopAt || nowMs
      if (fix) trackRide(run, route, stops, fix, nowMs)
    }
    if (next.kind === 'walkEnd') run.walkEndStartedAt = nowMs
    if (next.kind === 'walk') run.walkStartM = fix ? distanceTo(fix, boarding(journey.legs[next.leg]!)) : 0
    if (legOf(next) !== legOf(before)) {
      run.lastDepartureMinutes = null
      run.atStopAt = 0
    }
  }
  if (fix) run.seenFix = fixKey(fix)
  const legDeparture = changed && legOf(next) !== legOf(before) ? null : departure
  const parts = journeyParts(journey)
  const total = Math.max(parts.reduce((acc, part) => acc + part.minutes, 0), 1)
  const done = doneMinutes(journey, run, parts, fix)
  const described = describe(journey, run, fix, legDeparture)
  const alerted = described.alert && !run.alerted
  if (alerted) {
    run.alerted = true
    run.stepIndex++
  }
  return {
    step: {
      ...described,
      progress: Math.min(Math.max(done / total, 0), 1),
      etaAt: plannedArrival(journey, run, legDeparture, nowMs) ?? arrivalAt(run, parts, done, legDeparture, nowMs),
      stepIndex: run.stepIndex,
    },
    changed: changed || alerted,
    alerted,
    done: next.kind === 'done',
  }
}
