import {describe, expect, it} from 'vitest'
import {RouteLine, ridePath} from '@/utils/rideLine.ts'

const lat = 46.77
const m = 1 / (111_320 * Math.cos((lat * Math.PI) / 180))
const east = (meters: number, north = 0): [number, number] => [lat + north / 110_540, 23.58 + meters * m]
const point = ([pLat, pLon]: [number, number]) => ({lat: pLat, lon: pLon})

// Out east 1 km, then back west on the next street, 40 m north.
const path = [east(0), east(1000), east(1000, 40), east(0, 40)]

describe('a ride line', () => {
  it('finds a spot only within the stretch it is given', () => {
    const line = new RouteLine(path)
    const [pLat, pLon] = east(300, 38)
    expect(line.locate(pLat, pLon)!.along).toBeCloseTo(1740, -1)
    expect(line.locate(pLat, pLon, 0, 500)!.along).toBeCloseTo(300, -1)
  })

  it('cuts the ride out of the line’s shape', () => {
    const stops = [east(0), east(500), east(1000), east(500, 40)].map(point)
    const cut = new RouteLine(ridePath(path, stops, 1, 3))
    expect(cut.length).toBeCloseTo(1040, -1)
    const [sLat, sLon] = east(500)
    expect(cut.locate(sLat, sLon)!.along).toBeCloseTo(0, 0)
  })
})
