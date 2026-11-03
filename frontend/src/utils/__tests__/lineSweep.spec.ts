import {describe, expect, it} from 'vitest'
import {createExpression, latest} from '@maplibre/maplibre-gl-style-spec'
import {solid, step} from '@/utils/lineSweep.ts'

const valid = (paint: Parameters<typeof step>[0]) => createExpression(step(paint), latest.paint_line['line-gradient']).result

describe('step', () => {
  it('is a gradient MapLibre accepts for a single colour', () => {
    expect(valid(solid('rgba(0, 0, 0, 0)'))).toBe('success')
  })

  it('is a gradient MapLibre accepts for several colours', () => {
    expect(valid([{at: 0, color: '#34a853'}, {at: 0.4, color: '#ea4335'}])).toBe('success')
  })
})
