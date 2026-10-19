import {describe, expect, it} from 'vitest'
import type {RouteChange} from '@/stores/routeUpdates'
import {buildNewsFeed} from '@/utils/newsFeed.ts'

const t = (key: string) => key

const change = (id: number, detected: Date): RouteChange => ({
  id,
  route_short_name: '24B',
  route_id: 13,
  route_color: '#A119D7',
  source: 'timetable',
  detected_at: detected.getTime(),
  changes: [],
})

describe('news feed', () => {
  it('lists line changes newest first, by day', () => {
    const now = new Date(2026, 9, 10, 18, 0)
    const feed = buildNewsFeed(
      [change(2, new Date(2026, 9, 3, 12, 0)), change(1, new Date(2026, 9, 10, 9, 0)), change(3, new Date(2026, 9, 9, 7, 0))],
      now,
      'ro',
      t,
    )
    expect(feed.map((e) => [e.change.id, e.date])).toEqual([
      [1, 'newsToday'],
      [3, 'newsYesterday'],
      [2, '3 oct.'],
    ])
  })
})
