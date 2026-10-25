import type {Shape} from '@/types/tranzy.ts'
import {calculateBearing, haversineMeters} from '@/utils/geo.ts'
import type {LatLng} from '@/utils/mapMarkers.ts'

const MAX_OFF_ROUTE_M = 150
const MAX_MPS = 20
const ACCEL = 1.2
const BRAKE = 2.5
const CATCH_UP_S = 30
const HOLD_BACK_S = 40
const COAST_S = 8
const JUMP_M = 400
const HEADING_SPAN_M = 15
const BACKTRACK_M = 60
const TREND_WEIGHT = 0.3
const LATE_SHARE = 0.75

const clamp = (x: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, x))

export type Path = {lat: number[]; lng: number[]; at: number[]; length: number}

const paths = new WeakMap<Shape[], Path>()

export function pathOf(shape: Shape[]): Path {
  let path = paths.get(shape)
  if (path) return path
  const lat = shape.map((pt) => pt.shape_pt_lat)
  const lng = shape.map((pt) => pt.shape_pt_lon)
  const at = [0]
  for (let i = 1; i < shape.length; i++) at.push(at[i - 1]! + haversineMeters(lat[i - 1]!, lng[i - 1]!, lat[i]!, lng[i]!))
  path = {lat, lng, at, length: at.at(-1)!}
  paths.set(shape, path)
  return path
}

export function pointAt({lat, lng, at}: Path, s: number): LatLng {
  let lo = 0
  let hi = at.length - 1
  if (s <= 0 || hi === 0) return {lat: lat[0]!, lng: lng[0]!}
  if (s >= at[hi]!) return {lat: lat[hi]!, lng: lng[hi]!}
  while (hi - lo > 1) {
    const mid = (lo + hi) >> 1
    if (at[mid]! <= s) lo = mid
    else hi = mid
  }
  const k = (s - at[lo]!) / (at[hi]! - at[lo]! || 1)
  return {lat: lat[lo]! + (lat[hi]! - lat[lo]!) * k, lng: lng[lo]! + (lng[hi]! - lng[lo]!) * k}
}

export function headingAt(path: Path, s: number): number {
  const from = pointAt(path, s - HEADING_SPAN_M)
  const to = pointAt(path, s + HEADING_SPAN_M)
  return calculateBearing(from.lat, from.lng, to.lat, to.lng)
}

export function locate(path: Path, {lat, lng}: LatLng, from = 0, to = Infinity, heading?: number): {s: number; off: number} | null {
  const k = Math.cos((lat * Math.PI) / 180)
  const hx = heading === undefined ? 0 : Math.sin((heading * Math.PI) / 180)
  const hy = heading === undefined ? 0 : Math.cos((heading * Math.PI) / 180)
  let best: {s: number; d: number; i: number; t: number} | null = null
  for (let i = 0; i < path.at.length - 1; i++) {
    if (path.at[i + 1]! < from || path.at[i]! > to) continue
    const ax = path.lng[i]! * k,
      ay = path.lat[i]!
    const dx = path.lng[i + 1]! * k - ax,
      dy = path.lat[i + 1]! - ay
    if (heading !== undefined && dx * hx + dy * hy < 0) continue
    const len = dx * dx + dy * dy
    const t = len ? clamp(((lng * k - ax) * dx + (lat - ay) * dy) / len, 0, 1) : 0
    const d = (ax + dx * t - lng * k) ** 2 + (ay + dy * t - lat) ** 2
    if (!best || d < best.d) best = {s: path.at[i]! + (path.at[i + 1]! - path.at[i]!) * t, d, i, t}
  }
  if (!best) return null
  const i = best.i
  const off = haversineMeters(lat, lng, path.lat[i]! + (path.lat[i + 1]! - path.lat[i]!) * best.t, path.lng[i]! + (path.lng[i + 1]! - path.lng[i]!) * best.t)
  return off <= MAX_OFF_ROUTE_M ? {s: best.s, off} : null
}

export class VehicleTrack {
  s = 0
  v = 0
  private fixes: {s: number; ms: number}[] = []
  private pace = 0
  private gapMs = 30_000
  private lateMs = 30_000
  private lastMs = 0

  constructor(readonly path: Path) {}

  fix(at: LatLng, fixMs: number, speedKmh: number, heading: number, nowMs: number): boolean {
    const last = this.fixes.at(-1)
    if (last && fixMs <= last.ms) return true
    const spot = last
      ? (locate(this.path, at, last.s - MAX_OFF_ROUTE_M, last.s + (MAX_MPS * (fixMs - last.ms)) / 1000 + MAX_OFF_ROUTE_M) ?? locate(this.path, at))
      : (locate(this.path, at, 0, Infinity, heading) ?? locate(this.path, at))
    if (!spot || (last && spot.s < last.s - BACKTRACK_M)) return false
    if (last) {
      this.pace += (clamp(((spot.s - last.s) * 1000) / (fixMs - last.ms), 0, MAX_MPS) - this.pace) * TREND_WEIGHT
      this.gapMs = this.gapMs * 0.7 + (fixMs - last.ms) * 0.3
      this.lateMs = this.lateMs * 0.7 + (nowMs - fixMs) * 0.3
    } else {
      this.pace = clamp(speedKmh / 3.6, 0, MAX_MPS)
      this.lateMs = nowMs - fixMs
    }
    this.fixes = [...this.fixes.slice(-3), {s: spot.s, ms: fixMs}]
    if (!last) ({s: this.s, v: this.v} = this.target(nowMs))
    return true
  }

  target(nowMs: number): {s: number; v: number} {
    const t = nowMs - this.lateMs * LATE_SHARE
    const last = this.fixes.at(-1)!
    if (t < last.ms) {
      let i = this.fixes.length - 1
      while (i >= 0 && this.fixes[i]!.ms > t) i--
      if (i < 0) return {s: this.fixes[0]!.s, v: 0}
      const a = this.fixes[i]!,
        b = this.fixes[i + 1]!
      const v = ((b.s - a.s) * 1000) / (b.ms - a.ms)
      return {s: a.s + (v * (t - a.ms)) / 1000, v: Math.max(0, v)}
    }
    const age = (t - last.ms) / 1000
    const horizon = clamp(this.lateMs + this.gapMs, 10_000, 120_000) / 1000
    const coast = Math.exp(-Math.max(0, age - horizon) / COAST_S)
    const run = Math.min(age, horizon) + COAST_S * (1 - coast)
    return {s: Math.min(last.s + this.pace * run, this.path.length), v: this.pace * coast}
  }

  step(nowMs: number): boolean {
    const dt = (nowMs - this.lastMs) / 1000
    this.lastMs = nowMs
    const target = this.target(nowMs)
    const err = target.s - this.s
    if (Math.abs(err) > JUMP_M) {
      ;({s: this.s, v: this.v} = target)
      return true
    }
    if (dt > 1 || dt < 0) {
      this.s = Math.max(this.s, target.s)
      this.v = target.v
      return false
    }
    const want = clamp(target.v + err / (err > 0 ? CATCH_UP_S : HOLD_BACK_S), 0, MAX_MPS)
    this.v = Math.max(0, this.v + clamp(want - this.v, -BRAKE * dt, ACCEL * dt))
    this.s = Math.min(this.s + this.v * dt, this.path.length)
    return false
  }

  get position(): LatLng {
    return pointAt(this.path, this.s)
  }

  get heading(): number {
    return headingAt(this.path, this.s)
  }
}
