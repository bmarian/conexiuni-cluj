import {describe, expect, it} from 'vitest'
import {emptyHints, type HintContext, type HintLine, markShown, openLineFromStop, pickHint, readHints, visitStop} from '@/utils/hints.ts'

const ctx = (over: Partial<HintContext> = {}): HintContext => ({
  stop: 'cluj:1',
  day: '2026-11-13',
  hour: 7,
  saved: false,
  grouped: () => false,
  canRemind: false,
  ...over,
})

const days = (n: number) => Array.from({length: n}, (_, i) => `2026-11-${String(11 + i).padStart(2, '0')}`)
const visits = (n: number, hour = 7) => days(n).reduce((s, day) => visitStop(s, 'cluj:1', day, hour), emptyHints())
const line = (routeId: number): HintLine => ({routeId, direction: '0', platform: 1})

describe('hints', () => {
  it('suggests saving a stop opened on three different days', () => {
    expect(pickHint(visits(2), ctx())).toBeNull()
    expect(pickHint(visitStop(visits(2), 'cluj:1', '2026-11-12', 7), ctx())).toBeNull()
    expect(pickHint(visits(3), ctx())).toEqual({id: 'save'})
    expect(pickHint(visits(3), ctx({saved: true}))).toBeNull()
  })

  it('suggests a group for two lines each opened from the stop on two days', () => {
    let s = emptyHints()
    for (const day of days(2)) s = openLineFromStop(openLineFromStop(s, 'cluj:1', line(24), day), 'cluj:1', line(35), day)
    s = openLineFromStop(s, 'cluj:2', line(9), '2026-11-11')
    expect(pickHint(s, ctx())).toEqual({id: 'group', lines: [line(24), line(35)]})
    expect(pickHint(s, ctx({grouped: () => true}))).toBeNull()
    expect(pickHint(openLineFromStop(emptyHints(), 'cluj:1', line(24), '2026-11-11'), ctx())).toBeNull()
  })

  it('suggests a reminder only where reminders exist, for the same hour on three days', () => {
    expect(pickHint(visits(3), ctx({saved: true, canRemind: true}))).toEqual({id: 'remind'})
    expect(pickHint(visits(3, 8), ctx({saved: true, canRemind: true}))).toBeNull()
  })

  it('shows each tip once, and at most one a day', () => {
    const shown = markShown(visits(3), 'save', '2026-11-13')
    expect(pickHint(shown, ctx())).toBeNull()
    expect(pickHint(shown, ctx({day: '2026-11-14', saved: false}))).toBeNull()
    expect(pickHint(shown, ctx({day: '2026-11-14', saved: true, canRemind: true}))).toEqual({id: 'remind'})
  })

  it('reads back what it saves and drops anything else', () => {
    const s = markShown(visits(3), 'save', '2026-11-13')
    expect(readHints(JSON.parse(JSON.stringify(s)))).toEqual(s)
    expect(readHints({shown: ['nope', 'group'], days: {a: ['x', 3]}})).toEqual({shown: ['group'], lastDay: undefined, days: {a: ['x']}})
    expect(readHints(null)).toEqual(emptyHints())
  })
})
