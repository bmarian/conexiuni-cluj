import {describe, expect, it} from 'vitest'
import {shiftInto} from '@/utils/mapMarkers.ts'

describe('shiftInto', () => {
  it('leaves a span that already fits', () => {
    expect(shiftInto(20, 120, 8, 394)).toBe(0)
  })

  it('pulls a span back from the right edge', () => {
    expect(shiftInto(170, 446, 8, 394)).toBe(-52)
  })

  it('pushes a span away from the left edge', () => {
    expect(shiftInto(-30, 100, 8, 394)).toBe(38)
  })

  it('aligns to the left edge when the span is wider than the area', () => {
    expect(shiftInto(-30, 500, 8, 394)).toBe(38)
  })
})
