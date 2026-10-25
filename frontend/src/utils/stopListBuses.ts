import {locate, type Path} from '@/utils/vehicleMotion.ts'

type Point = {stop_lat: number; stop_lon: number}

// Each stop is searched after the one before, so a line passing a street twice finds the right side.
export function stopsAlong(path: Path, stops: Point[]): (number | null)[] {
  let from = 0
  return stops.map((stop) => {
    const at = locate(path, {lat: stop.stop_lat, lng: stop.stop_lon}, from)
    if (at) from = at.s
    return at?.s ?? null
  })
}

// Where a point s metres along the path sits in the stop list: 2.5 is halfway from the third stop to the fourth.
export function rowAlong(stopsAt: (number | null)[], s: number): number {
  const known = stopsAt.flatMap((at, row) => (at === null ? [] : [{row, at}]))
  if (!known.length) return 0
  if (s <= known[0]!.at) return known[0]!.row
  for (let i = 1; i < known.length; i++) {
    const a = known[i - 1]!
    const b = known[i]!
    if (s < b.at) return a.row + ((b.row - a.row) * (s - a.at)) / (b.at - a.at || 1)
  }
  return known.at(-1)!.row
}
