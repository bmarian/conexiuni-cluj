import {describe, expect, it} from 'vitest'
import type {RouteChange} from '@/stores/routeUpdates'
import {buildNewsFeed, newsSource, parseNewsDate} from '@/utils/newsFeed.ts'

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

const news = (date: string, title = date) => ({url: `https://www.ctpcj.ro/stiri/${title}`, date, title})

describe('news feed', () => {
  it('reads the dates CTP writes', () => {
    expect(parseNewsDate('9 Oct 2026')).toEqual(new Date(2026, 9, 9))
    expect(parseNewsDate('30 sept. 2026')).toEqual(new Date(2026, 8, 30))
    expect(parseNewsDate('1 mai 2026')).toEqual(new Date(2026, 4, 1))
    expect(parseNewsDate('09.10.2026')).toEqual(new Date(2026, 9, 9))
    expect(parseNewsDate('2026-10-09T08:00:00Z')).toEqual(new Date(2026, 9, 9))
    expect(parseNewsDate('soon')).toBeNull()
  })

  it('mixes line changes and CTP news newest first', () => {
    const now = new Date(2026, 9, 10, 18, 0)
    const feed = buildNewsFeed(
      [change(1, new Date(2026, 9, 10, 9, 0)), change(2, new Date(2026, 9, 3, 12, 0))],
      [news('9 Oct 2026'), news('10 Oct 2026'), news('30 Sep 2025')],
      now,
      'ro',
      t,
    )
    expect(feed.map((e) => [e.kind, e.date])).toEqual([
      ['change', 'newsToday'],
      ['news', 'newsToday'],
      ['news', 'newsYesterday'],
      ['change', '3 oct.'],
      ['news', '30 sept. 2025'],
    ])
  })

  it('keeps unreadable news dates at the end as CTP wrote them', () => {
    const feed = buildNewsFeed([], [news('soon'), news('9 Oct 2026')], new Date(2026, 9, 20), 'en', t)
    expect(feed.map((e) => e.date)).toEqual(['9 Oct', 'soon'])
  })

  it('names the source by its domain', () => {
    expect(newsSource('https://www.ctpcj.ro/index.php/ro/despre-noi/stiri/x/1880')).toBe('ctpcj.ro')
    expect(newsSource('not a url')).toBe('')
  })
})
