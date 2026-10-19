import {describe, expect, it} from 'vitest'
import type {Update} from '@/stores/routeUpdates'
import {buildNewsFeed, newsSource} from '@/utils/newsFeed.ts'

const t = (key: string) => key

const change = (id: number, detected: Date): Update => ({
  kind: 'change',
  at: detected.getTime(),
  change: {
    id,
    route_short_name: '24B',
    route_id: 13,
    route_color: '#A119D7',
    source: 'timetable',
    detected_at: detected.getTime(),
    changes: [],
  },
})

const news = (url: string, at: Date): Update => ({kind: 'news', at: at.getTime(), news: {title: 'Linia 21 deviată', url}})

describe('news feed', () => {
  it('labels each update by day and keeps the server order', () => {
    const now = new Date(2026, 9, 10, 18, 0)
    const feed = buildNewsFeed(
      [
        change(1, new Date(2026, 9, 10, 9, 0)),
        news('https://www.ctpcj.ro/stiri/1880', new Date(2026, 9, 9, 0, 0)),
        change(2, new Date(2026, 9, 3, 12, 0)),
      ],
      now,
      'ro',
      t,
    )
    expect(feed.map((e) => [e.key, e.date])).toEqual([
      ['change-1', 'newsToday'],
      ['news-https://www.ctpcj.ro/stiri/1880', 'newsYesterday'],
      ['change-2', '3 oct.'],
    ])
  })

  it('names the site a news item comes from', () => {
    expect(newsSource('https://www.ctpcj.ro/stiri/1880')).toBe('ctpcj.ro')
    expect(newsSource('not a url')).toBe('')
  })
})
