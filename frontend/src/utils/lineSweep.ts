import type {ExpressionSpecification, Map as MapLibreMap} from 'maplibre-gl'

// In step with the badge's heart; the source needs lineMetrics.
export const SWEEP_MS = 1300

const sweeps = new Map<string, number>()

const easeInOut = (t: number) => (t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2)

export function stopSweep(m: MapLibreMap, id: string) {
  const frame = sweeps.get(id)
  if (frame === undefined) return
  cancelAnimationFrame(frame)
  sweeps.delete(id)
  if (m.getLayer(id)) m.setPaintProperty(id, 'line-gradient', undefined)
}

const gradient = (from: string, to: string, done: number, backwards: boolean): ExpressionSpecification =>
  backwards ? ['step', ['line-progress'], from, 1 - done, to] : ['step', ['line-progress'], to, done, from]

// Paints [to] over [from] along the line, from its first point or, backwards, from its last.
export function sweepLine(m: MapLibreMap, id: string, from: string, to: string, backwards = false) {
  stopSweep(m, id)
  m.setPaintProperty(id, 'line-gradient', gradient(from, to, 0, backwards))
  const start = performance.now()
  const frame = (now: number) => {
    const t = (now - start) / SWEEP_MS
    if (t >= 1 || !m.getLayer(id)) return stopSweep(m, id)
    m.setPaintProperty(id, 'line-gradient', gradient(from, to, easeInOut(t), backwards))
    sweeps.set(id, requestAnimationFrame(frame))
  }
  sweeps.set(id, requestAnimationFrame(frame))
}
