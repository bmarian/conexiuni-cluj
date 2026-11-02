import {describe, expect, it} from 'vitest'
import {TRAFFIC_COLORS, trafficFeatures, type TrafficNow} from '@/utils/traffic.ts'

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

const now: TrafficNow = {
  lines: {
    '18_0': [{path: encodePolyline([[46.77, 23.59], [46.771, 23.592]]), band: 2}],
    '18_1': [{path: encodePolyline([[46.78, 23.6], [46.781, 23.601]]), band: 0}],
  },
}

describe('traffic on the drawn lines', () => {
  it('draws the stretches of the drawn lines in lon/lat with their band', () => {
    const {features} = trafficFeatures(now, ['18_0', '18_0', '92_0'])
    expect(features).toHaveLength(1)
    expect(features[0].properties).toEqual({band: 2})
    expect(features[0].geometry.coordinates).toEqual([
      [23.59, 46.77],
      [23.592, 46.771],
    ])
  })

  it('has a colour per band', () => {
    expect(TRAFFIC_COLORS).toHaveLength(4)
  })
})
