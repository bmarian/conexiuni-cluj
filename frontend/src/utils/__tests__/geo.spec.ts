import {describe, expect, it} from 'vitest'
import {convexHull, roundedHull} from '@/utils/geo.ts'

describe('hulls', () => {
  it('drops the points inside', () => {
    const hull = convexHull([[0, 0], [2, 0], [1, 1], [2, 2], [0, 2], [1, 0]])
    expect(hull).toHaveLength(4)
    expect(hull).not.toContainEqual([1, 1])
  })

  it('gives two stops a closed shape around both', () => {
    const ring = roundedHull([[23.59, 46.77], [23.6, 46.77]], 0.001)
    expect(ring[0]).toEqual(ring.at(-1))
    const lngs = ring.map(([lng]) => lng)
    const lats = ring.map(([, lat]) => lat)
    expect(Math.min(...lngs)).toBeCloseTo(23.589)
    expect(Math.max(...lngs)).toBeCloseTo(23.601)
    expect(Math.max(...lats) - 46.77).toBeCloseTo(0.001 * Math.cos(46.77 * Math.PI / 180))
  })
})
