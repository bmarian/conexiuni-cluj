import {describe, expect, it} from 'vitest'
import {easeOutCubic, flightMs, linear} from '@/utils/markerMotion.ts'

describe('marker motion', () => {
  it('eases out and ends exactly at the target', () => {
    expect(easeOutCubic(0)).toBe(0)
    expect(easeOutCubic(1)).toBe(1)
    expect(easeOutCubic(0.5)).toBeGreaterThan(0.5)
    expect(linear(0.3)).toBe(0.3)
  })

  it('flies short hops quickly and caps long ones', () => {
    expect(flightMs(0, 0)).toBe(305)
    expect(flightMs(0, 2)).toBe(570)
    expect(flightMs(4000, 3)).toBe(955)
    expect(flightMs(800, 1)).toBeGreaterThan(flightMs(200, 1))
  })
})
