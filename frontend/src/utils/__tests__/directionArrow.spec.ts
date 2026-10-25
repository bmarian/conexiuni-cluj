import {describe, expect, it} from 'vitest'
import {ARROW_MS, arrowKeyframes, arrowPose} from '@/utils/directionArrow.ts'

const turned = (a: number) => ((a % 360) + 360) % 360

describe('direction arrow', () => {
  it('starts on its spot facing the name and lands there facing the heading', () => {
    for (const [rest, heading] of [
      [90, 292],
      [270, 112],
      [90, 90],
      [270, 271],
      [90, 0],
    ] as const) {
      const [x0, y0, a0] = arrowPose(rest, heading, 0)
      expect(Math.hypot(x0, y0)).toBe(0)
      expect(a0).toBe(rest)
      const [x, y, a] = arrowPose(rest, heading, ARROW_MS)
      expect(Math.hypot(x, y)).toBeLessThan(1e-9)
      expect(turned(a)).toBeCloseTo(turned(heading), 6)
    }
  })

  it('taps towards the name twice, back on its spot in between', () => {
    expect(arrowPose(270, 112, 70)[0]).toBeLessThan(-4)
    expect(Math.abs(arrowPose(270, 112, 239)[0])).toBeLessThan(0.01)
    expect(arrowPose(270, 112, 310)[0]).toBeLessThan(-4)
    expect(arrowPose(90, 292, 70)[0]).toBeGreaterThan(4)
  })

  it('circles round its spot without spinning between frames', () => {
    for (const rest of [90, 270]) {
      const frames = arrowKeyframes(rest, 112).map((f) =>
        String(f.transform)
          .match(/-?\d[\d.e-]*/g)!
          .map(Number),
      )
      for (const axis of [0, 1]) {
        expect(Math.min(...frames.map((f) => f[axis]!))).toBeLessThan(-3)
        expect(Math.max(...frames.map((f) => f[axis]!))).toBeGreaterThan(3)
      }
      for (let i = 1; i < frames.length; i++) expect(Math.abs(frames[i]![2]! - frames[i - 1]![2]!)).toBeLessThan(30)
    }
  })
})
