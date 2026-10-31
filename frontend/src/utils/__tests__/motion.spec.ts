import {afterEach, describe, expect, it, vi} from 'vitest'
import {applyPaces, spring, SPRING_PACE} from '@/utils/motion.ts'

describe('motion', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('has a real duration for a spring whatever the stylesheet says', () => {
    vi.stubGlobal('document', {documentElement: {}})
    vi.stubGlobal('getComputedStyle', () => ({getPropertyValue: () => ''}))
    expect(spring('pop').duration).toBe(635 * SPRING_PACE)
  })

  it('hands the CSS every spring length', () => {
    const set = new Map<string, string>()
    applyPaces({style: {setProperty: (name: string, value: string) => set.set(name, value)}} as unknown as HTMLElement)
    expect(set.get('--spring-press-ms')).toBe(`${755 * SPRING_PACE}ms`)
    expect(set.get('--pace-long')).toBe('0.5')
  })
})
