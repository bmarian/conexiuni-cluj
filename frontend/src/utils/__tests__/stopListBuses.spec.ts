import {describe, expect, it} from 'vitest'
import {haversineMeters} from '@/utils/geo.ts'
import {rowAlong, stopsAlong} from '@/utils/stopListBuses.ts'
import {pathThrough} from '@/utils/vehicleMotion.ts'

const M_PER_LNG = haversineMeters(46.77, 0, 46.77, 1)
const lngAt = (m: number) => 23.55 + m / M_PER_LNG
const path = pathThrough([46.77, 46.77], [lngAt(0), lngAt(3000)])
const stop = (m: number) => ({stop_lat: 46.77, stop_lon: lngAt(m)})

describe('rowAlong', () => {
  const stops = stopsAlong(path, [stop(0), stop(1000), stop(2000), stop(3000)])

  it('puts a point between the stops it lies between', () => {
    expect(rowAlong(stops, 1500)).toBeCloseTo(1.5, 2)
    expect(rowAlong(stops, 2000)).toBeCloseTo(2, 2)
  })

  it('keeps a point before the first stop or past the last one on the list', () => {
    expect(rowAlong(stops, -20)).toBe(0)
    expect(rowAlong(stops, 3200)).toBe(3)
  })

  it('skips a stop that is not on the path', () => {
    expect(rowAlong([0, null, 2000], 1000)).toBeCloseTo(1, 5)
  })
})
