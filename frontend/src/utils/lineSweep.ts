import type {ExpressionSpecification, Map as MapLibreMap} from 'maplibre-gl'
import {paced} from '@/utils/motion.ts'

// In step with the badge's heart; the source needs lineMetrics.
export const SWEEP_MS = paced(1130)

export const CLEAR = 'rgba(0, 0, 0, 0)'

// A line's colours along it: each colour runs from its share of the length up to the next one's.
export type Paint = {at: number; color: string}[]

export const solid = (color: string): Paint => [{at: 0, color}]

const sweeps = new Map<string, number>()
const paints = new Map<string, Paint>()
// Undefined while the grown line waits to start.
const growing = new Map<string, number | undefined>()

export const easeInOut = (t: number) => (t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2)

// MapLibre rejects a step without stops.
export const step = (paint: Paint): ExpressionSpecification => {
  const stops = paint.length > 1 ? paint.slice(1) : [{at: 0, color: paint[0]!.color}]
  return ['step', ['line-progress'], paint[0]!.color, ...stops.flatMap(({at, color}) => [at, color])] as ExpressionSpecification
}

const colorAt = (paint: Paint, at: number) => paint.reduce((color, stop) => (stop.at <= at ? stop.color : color), paint[0]!.color)

const settle = (m: MapLibreMap, id: string) => {
  const paint = paints.get(id)
  m.setPaintProperty(id, 'line-gradient', paint && step(paint))
}

export function stopSweep(m: MapLibreMap, id: string) {
  const frame = sweeps.get(id)
  if (frame === undefined) return
  cancelAnimationFrame(frame)
  sweeps.delete(id)
  growing.delete(id)
  if (m.getLayer(id)) settle(m, id)
}

export function paintLine(m: MapLibreMap, id: string, paint?: Paint) {
  if (paint) paints.set(id, paint)
  else paints.delete(id)
  if (!sweeps.has(id) && m.getLayer(id)) settle(m, id)
}

function animate(m: MapLibreMap, id: string, gradientAt: (done: number) => ExpressionSpecification, start = performance.now()) {
  stopSweep(m, id)
  m.setPaintProperty(id, 'line-gradient', gradientAt(0))
  const frame = (now: number) => {
    const t = (now - start) / SWEEP_MS
    if (t >= 1 || !m.getLayer(id)) return stopSweep(m, id)
    m.setPaintProperty(id, 'line-gradient', gradientAt(easeInOut(t)))
    sweeps.set(id, requestAnimationFrame(frame))
  }
  sweeps.set(id, requestAnimationFrame(frame))
}

// [to] up to [done] along the line and [from] past it; backwards, [from] up to 1 - done and [to] past it.
const gradient = (from: Paint, to: Paint, done: number, backwards: boolean): ExpressionSpecification => {
  const cut = backwards ? 1 - done : done
  const [head, tail] = backwards ? [from, to] : [to, from]
  return step([...head.filter((stop) => stop.at < cut), {at: cut, color: colorAt(tail, cut)}, ...tail.filter((stop) => stop.at > cut)])
}

export function sweepLine(m: MapLibreMap, id: string, from: string, to: string, backwards = false) {
  const was = paints.get(id) ?? solid(from)
  animate(m, id, (done) => gradient(was, paints.get(id) ?? solid(to), done, backwards))
}

export function growLine(m: MapLibreMap, id: string, color: string): (start?: number) => void {
  stopSweep(m, id)
  const paint = () => paints.get(id) ?? solid(color)
  m.setPaintProperty(id, 'line-gradient', gradient(solid(CLEAR), paint(), 0, false))
  // So stopSweep clears the hidden line too.
  sweeps.set(id, 0)
  growing.set(id, undefined)
  return (start = performance.now()) => {
    animate(m, id, (done) => gradient(solid(CLEAR), paint(), done, false), start)
    growing.set(id, start)
  }
}

export function followGrowth(m: MapLibreMap, id: string, lead: string) {
  if (!growing.has(lead)) return
  const start = growLine(m, id, CLEAR)
  const at = growing.get(lead)
  if (at !== undefined) start(at)
}
