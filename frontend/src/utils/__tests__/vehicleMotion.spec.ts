import {describe, expect, it} from 'vitest'
import type {Shape} from '@/types/tranzy.ts'
import {haversineMeters} from '@/utils/geo.ts'
import {headingAt, locate, pathOf, pointAt, VehicleTrack} from '@/utils/vehicleMotion.ts'

const M_PER_LNG = haversineMeters(46.77, 0, 46.77, 1)
const M_PER_LAT = haversineMeters(0, 0, 1, 0)
const east = (m: number, north = 0) => ({lat: 46.77 + north / M_PER_LAT, lng: 23.55 + m / M_PER_LNG})
const shape = (pts: {lat: number; lng: number}[]): Shape[] =>
  pts.map(({lat, lng}, i) => ({shape_id: 't', shape_pt_lat: lat, shape_pt_lon: lng, shape_pt_sequence: i, shape_dist_traveled: -1}))
const line = pathOf(shape([0, 1000, 2000, 4000, 8000].map((m) => east(m))))

function drive(track: VehicleTrack, from: number, ms: number) {
  const frames: {s: number; v: number}[] = []
  for (let now = from + 16; now <= from + ms; now += 16) {
    track.step(now)
    frames.push({s: track.s, v: track.v})
  }
  return frames
}

describe('paths', () => {
  it('measures along the line and clamps past its ends', () => {
    expect(line.length).toBeCloseTo(8000, -1)
    expect(pointAt(line, 3000).lng).toBeCloseTo(east(3000).lng, 6)
    expect(pointAt(line, -5)).toEqual(east(0))
    expect(pointAt(line, 1e6).lng).toBeCloseTo(east(8000).lng, 9)
    expect(headingAt(line, 500)).toBeCloseTo(90, 0)
  })

  it('projects a fix onto the line and drops one too far off it', () => {
    expect(locate(line, east(2500, 20))!.s).toBeCloseTo(2500, -1)
    expect(locate(line, east(2500, 400))).toBeNull()
  })

  it('takes the leg the bus faces where the route doubles back', () => {
    const loop = pathOf(shape([east(0), east(1000), east(1000, 15), east(0, 15)]))
    expect(locate(loop, east(400, 8), 0, Infinity, 90)!.s).toBeLessThan(1000)
    expect(locate(loop, east(400, 8), 0, Infinity, 270)!.s).toBeGreaterThan(1000)
  })
})

describe('VehicleTrack', () => {
  it('never backs up when a fix lands behind the drawn bus', () => {
    const track = new VehicleTrack(line)
    let now = 1e9
    track.fix(east(1000), now - 30_000, 36, 90, now)
    const before = drive(track, now, 30_000)
    now += 30_000
    track.fix(east(1050), now - 30_000, 0, 90, now)
    const after = drive(track, now, 60_000)
    const all = [...before, ...after]
    expect(all.every((f, i) => i === 0 || f.s >= all[i - 1]!.s)).toBe(true)
    expect(after.at(-1)!.v).toBeLessThan(before.at(-1)!.v)
  })

  it('catches up on a fix ahead without leaping', () => {
    const track = new VehicleTrack(line)
    let now = 1e9
    track.fix(east(1000), now - 30_000, 0, 90, now)
    drive(track, now, 30_000)
    now += 30_000
    track.fix(east(1300), now - 30_000, 30, 90, now)
    const frames = drive(track, now, 120_000)
    const steps = frames.slice(1).map((f, i) => f.s - frames[i]!.s)
    expect(Math.max(...steps)).toBeLessThan(0.4)
    expect(Math.max(...frames.map((f) => f.v))).toBeLessThanOrEqual(20)
    expect(Math.abs(track.s - track.target(now + 120_000).s)).toBeLessThan(25)
  })

  it('coasts to a stop when fixes stop coming', () => {
    const track = new VehicleTrack(line)
    const now = 1e9
    track.fix(east(1000), now - 30_000, 36, 90, now)
    const frames = drive(track, now, 300_000)
    expect(frames.at(-1)!.v).toBeLessThan(0.05)
    expect(frames.at(-1)!.s - frames[frames.length - 3000]!.s).toBeLessThan(1)
    expect(frames.at(-1)!.s).toBeLessThan(2000)
  })

  it('gives up on a bus running backwards along its path', () => {
    const track = new VehicleTrack(line)
    const now = 1e9
    expect(track.fix(east(3000), now - 60_000, 30, 90, now)).toBe(true)
    expect(track.fix(east(2700), now - 30_000, 30, 90, now)).toBe(false)
  })

  it('relocates instead of driving a correction it cannot make up', () => {
    const track = new VehicleTrack(line)
    let now = 1e9
    track.fix(east(1000), now - 30_000, 0, 90, now)
    drive(track, now, 5000)
    now += 5000
    track.fix(east(3000), now - 30_000, 0, 90, now)
    expect(track.step(now + 16)).toBe(true)
    expect(track.s).toBe(track.target(now + 16).s)
    expect(track.s).toBeGreaterThan(2990)
  })
})
