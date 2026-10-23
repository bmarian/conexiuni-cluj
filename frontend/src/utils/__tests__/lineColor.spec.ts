import {describe, expect, it} from 'vitest'
import {hashedLineColor} from '@/utils/lineColor.ts'

describe('hashedLineColor', () => {
  it('gives each line the colour the backend used to', () => {
    const expected: Record<string, string> = {
      '25': '#462EE0', '35': '#DA771B', '24B': '#A119D7', 'M26N': '#32C322', '8L': '#2A52CB',
      '101': '#DD1DBA', ' 5n ': '#2552E4', 'Ș1': '#1FC16B', 'RoHu': '#DE3581', 'N104': '#70C328',
    }
    for (const [name, color] of Object.entries(expected)) expect(hashedLineColor(name), name).toBe(color)
  })
})
