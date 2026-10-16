import {describe, expect, it} from 'vitest'
import {
  clockAt, clujMinutes, freshProgress, GIVE_UP_MS, justNotify, leadMinutes, leaveInMinutes, stepFollow, targetMinutes,
  targetReading, walkFrom, walkMinutes, type FollowProgress, type FollowReading,
} from '@/utils/follow.ts'

const read = (minutes: number, live = false): FollowReading => ({minutes, live, later: []})
const T0 = Date.parse('2026-10-09T18:00:00+03:00')

function run(minutesSeq: (number | null)[], options: {target?: boolean, walk?: number | null} = {}) {
  let progress: FollowProgress = freshProgress()
  const out: {alerts: string[], end?: string, leaveAt: number | null}[] = []
  minutesSeq.forEach((minutes, i) => {
    const step = stepFollow(progress, minutes === null ? null : read(minutes), {
      target: options.target ?? false, walk: options.walk ?? null, nowMs: T0 + i * 60_000, startedAt: T0,
    })
    progress = step.progress
    out.push({alerts: step.alerts, end: step.end, leaveAt: step.leaveAt})
  })
  return out
}

describe('walking', () => {
  it('rounds up and is zero at the stop', () => {
    expect(walkMinutes(46.77, 23.59, 46.7702, 23.5901)).toBe(0)
    // About 1 km as the crow flies.
    expect(walkMinutes(46.77, 23.59, 46.779, 23.59)).toBe(16)
    expect(leadMinutes(12)).toBe(17)
    expect(leadMinutes(0)).toBe(5)
    expect(leadMinutes(null)).toBeNull()
    expect(walkFrom(null, 46.77, 23.59)).toBeNull()
    expect(walkFrom({latitude: 46.77, longitude: 23.59}, undefined, undefined)).toBeNull()
    expect(walkFrom({latitude: 46.77, longitude: 23.59}, 46.779, 23.59)).toBe(16)
  })

  it('only says the bus is coming at the stop or when it is too late to walk there', () => {
    expect(justNotify(0, 30)).toBe(true)
    expect(justNotify(12, 17)).toBe(true)
    expect(justNotify(12, 18)).toBe(false)
    expect(justNotify(null, 20)).toBe(true)
    expect(justNotify(null, 21)).toBe(false)
  })
})

describe('a followed bus', () => {
  it('alerts when close and when there, then ends once it has gone', () => {
    const steps = run([6, 4, 2, 1, 0, 9])
    expect(steps.map((s) => s.alerts)).toEqual([[], [], ['soon'], [], ['arrived'], []])
    expect(steps.at(-1)!.end).toBe('left')
  })

  it('does not alert on its first reading: the user just asked', () => {
    expect(run([1]).at(0)!.alerts).toEqual([])
    expect(run([0, 0]).flatMap((s) => s.alerts)).toEqual([])
  })

  it('ends when the bus was close and then drops out of the list', () => {
    const steps = run([3, 2, null])
    expect(steps[1]!.alerts).toEqual(['soon'])
    expect(steps[2]!.end).toBe('left')
  })

  it('says there are no more buses when none was ever close', () => {
    expect(run([null])[0]!.end).toBe('none')
  })

  it('stops after an hour and a half', () => {
    const step = stepFollow(freshProgress(), read(30), {target: false, walk: null, nowMs: T0 + GIVE_UP_MS + 1, startedAt: T0})
    expect(step.end).toBe('stopped')
  })

  it('tells you when to leave from how far you are, once', () => {
    const steps = run([20, 17, 15, 14, 2], {walk: 10})
    expect(steps.map((s) => s.alerts)).toEqual([[], [], ['leave'], [], ['soon']])
    expect(steps[0]!.leaveAt).toBe(T0 + 5 * 60_000)
    expect(leaveInMinutes(steps[0]!.leaveAt!, T0)).toBe(5)
    expect(steps[4]!.leaveAt).toBeNull()
  })

  it('has no leave time at the stop or without a location', () => {
    expect(run([20], {walk: 0})[0]!.leaveAt).toBeNull()
    expect(run([20], {walk: null})[0]!.leaveAt).toBeNull()
  })

  it('tells a reminder to leave straight away when it is already time', () => {
    expect(run([12], {target: true, walk: 10})[0]!.alerts).toEqual(['leave'])
    expect(run([null], {target: true})[0]!.end).toBe('left')
  })
})

describe('a reminder’s own bus', () => {
  it('only takes the live bus when it is ours', () => {
    expect(targetMinutes([4, 20], 5, 20, 3)).toBe(20)
    expect(targetMinutes([4, 20], 7, 4, 3)).toBe(7)
    expect(targetMinutes([-2], null, -2, 3)).toBe(0)
    expect(targetMinutes([], null, -5, 3)).toBeNull()
  })

  it('reads minutes from the target time in Cluj', () => {
    const now = Date.parse('2026-10-09T22:40:00+03:00')
    const target = Date.parse('2026-10-09T22:50:00+03:00')
    expect(clujMinutes(target)).toBe(22 * 60 + 50)
    expect(targetReading([10, 40], null, target, now, 3)).toEqual({minutes: 10, live: false, later: [40]})
    const late = Date.parse('2026-10-10T00:05:00+03:00')
    expect(targetReading([25, 85], null, late, now, 3)!.minutes).toBe(85)
    expect(clockAt(now, 25)).toBe('23:05')
  })
})
