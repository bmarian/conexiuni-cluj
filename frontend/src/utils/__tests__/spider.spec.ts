import {describe, expect, it} from 'vitest'
import {spiderLegs, spiderRadius} from '@/utils/spider.ts'

describe('spider legs', () => {
  it('puts two stations side by side', () => {
    const [a, b] = spiderLegs(2)
    expect(a).toEqual({x: 44, y: 0, side: 'right'})
    expect(b).toEqual({x: -44, y: 0, side: 'left'})
  })

  it('keeps every station at least an icon apart', () => {
    for (const count of [3, 5, 8, 9, 14, 30]) {
      const legs = spiderLegs(count)
      expect(legs).toHaveLength(count)
      legs.forEach((a, i) => legs.slice(i + 1).forEach((b) => expect(Math.hypot(a.x - b.x, a.y - b.y)).toBeGreaterThanOrEqual(28)))
    }
  })

  it('labels face away from the centre', () => {
    for (const {x, side} of spiderLegs(8)) expect(side).toBe(x < 0 ? 'left' : 'right')
  })

  it('measures the reach of the longest leg', () => {
    expect(spiderRadius(spiderLegs(2))).toBe(44)
    expect(spiderRadius(spiderLegs(20))).toBeGreaterThan(spiderRadius(spiderLegs(9)))
    expect(spiderRadius([])).toBe(0)
  })
})
