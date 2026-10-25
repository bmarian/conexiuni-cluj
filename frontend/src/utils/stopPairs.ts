import {haversineMeters} from '@/utils/geo.ts'

type Stop = {stop_id: number; stop_lat: number; stop_lon: number}

const apart = (a: Stop, b: Stop) => haversineMeters(a.stop_lat, a.stop_lon, b.stop_lat, b.stop_lon)

// A stop keeps its key in the other direction at the same station, or across the street: nearer than half the gap to its neighbours.
export function directionKeys(there: Stop[], back: Stop[], stationOf: (stop: Stop) => number): [string[], string[]] {
  const counts = new Map<number, number>()
  const thereKeys = there.map((stop) => {
    const station = stationOf(stop)
    const n = counts.get(station) ?? 0
    counts.set(station, n + 1)
    return `${station}-${n}`
  })
  const gap = (i: number) => Math.min(...[back[i - 1], back[i + 1]].filter((s): s is Stop => !!s).map((s) => apart(back[i]!, s)), Infinity)
  const pairs = back
    .flatMap((stop, j) =>
      there.map((other, i) => ({i, j, d: stationOf(stop) === stationOf(other) ? 0 : apart(stop, other)})).filter(({j: k, d}) => d === 0 || d < gap(k) / 2),
    )
    .sort((a, b) => a.d - b.d)
  const taken = new Set<number>()
  const backKeys: (string | undefined)[] = back.map(() => undefined)
  for (const {i, j} of pairs) {
    if (taken.has(i) || backKeys[j] !== undefined) continue
    taken.add(i)
    backKeys[j] = thereKeys[i]
  }
  return [thereKeys, backKeys.map((key, j) => key ?? `back-${back[j]!.stop_id}-${j}`)]
}
