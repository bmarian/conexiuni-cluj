import {describe, expect, it} from 'vitest'
import {type Jam, TRAFFIC_COLORS, trafficPaint, worstJam} from '@/utils/traffic.ts'

// The backend's polyline encoding, six decimals.
function encodePolyline(points: [number, number][]): string {
  let out = ''
  const prev = [0, 0]
  for (const point of points) {
    point.forEach((coord, i) => {
      const value = Math.round(coord * 1e6)
      let shifted = (value - prev[i]) << 1
      if (value - prev[i] < 0) shifted = ~shifted
      while (shifted >= 0x20) {
        out += String.fromCharCode((0x20 | (shifted & 0x1f)) + 63)
        shifted >>= 5
      }
      out += String.fromCharCode(shifted + 63)
      prev[i] = value
    })
  }
  return out
}

// A straight line east, four equal legs.
const line: [number, number][] = [
  [46.77, 23.59],
  [46.77, 23.6],
  [46.77, 23.61],
  [46.77, 23.62],
  [46.77, 23.63],
]
const leg = (from: number, to: number) => encodePolyline(line.slice(from, to + 1))

describe('traffic paint', () => {
  it('colours each stretch over its share of the line and leaves the rest in the line colour', () => {
    const paint = trafficPaint([{path: leg(0, 1), band: 2}, {path: leg(2, 3), band: 0}], line, '#123456')
    expect(paint.map((stop) => [Math.round(stop.at * 100), stop.color])).toEqual([
      [0, TRAFFIC_COLORS[2]],
      [25, '#123456'],
      [50, TRAFFIC_COLORS[0]],
      [75, '#123456'],
    ])
  })

  it('joins touching stretches of one band', () => {
    const paint = trafficPaint([{path: leg(0, 2), band: 1}, {path: leg(2, 4), band: 1}], line, '#123456')
    expect(paint).toEqual([{at: 0, color: TRAFFIC_COLORS[1]}])
  })

  it('is the line colour alone when nothing is measured', () => {
    expect(trafficPaint([], line, '#123456')).toEqual([{at: 0, color: '#123456'}])
  })
})

describe('worst jam', () => {
  const jam = (from: number, delay: number): Jam => ({from_stop_id: from, to_stop_id: from + 1, band: 2, delay_sec: delay})
  const traffic = {stretches: [], jams: [jam(1, 90), jam(3, 200), jam(5, 120)]}

  it('is the one costing the most on the trip', () => {
    expect(worstJam(traffic, [1, 2, 3, 4, 5, 6])?.from_stop_id).toBe(3)
  })

  it('counts only jams from the given stop on', () => {
    expect(worstJam(traffic, [1, 2, 3, 4, 5, 6], 4)?.from_stop_id).toBe(5)
    expect(worstJam(traffic, [1, 2, 3, 4, 5, 6], 6)).toBeUndefined()
  })
})
