import type {RouteChange} from '@/stores/routeUpdates'
import {formatChangeDate} from '@/utils/routeChanges'

type Translate = (key: string) => string

export interface NewsItem {
  url: string
  date: string
  title: string
}

export type NewsEntry =
  | { kind: 'change'; key: string; at: number; date: string; change: RouteChange }
  | { kind: 'news'; key: string; at: number; date: string; item: NewsItem }

const MONTHS = [['jan', 'ian'], ['feb'], ['mar'], ['apr'], ['may', 'mai'], ['jun', 'iun'], ['jul', 'iul'], ['aug'], ['sep'], ['oct'], ['nov', 'noi'], ['dec']]

// The date as CTP writes it ("9 Oct 2026", "09.10.2026", ISO); null when it can't be read.
export function parseNewsDate(text: string): Date | null {
  const clean = text.trim()
  let m = /^(\d{4})-(\d{2})-(\d{2})/.exec(clean)
  if (m) return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  m = /^(\d{1,2})\.(\d{1,2})\.(\d{4})$/.exec(clean)
  if (m) return new Date(Number(m[3]), Number(m[2]) - 1, Number(m[1]))
  m = /^(\d{1,2})\s+(\p{L}+)\.?\s+(\d{4})$/u.exec(clean)
  if (!m) return null
  const name = m[2]!.toLowerCase()
  const month = MONTHS.findIndex((prefixes) => prefixes.some((p) => name.startsWith(p)))
  return month === -1 ? null : new Date(Number(m[3]), month, Number(m[1]))
}

function startOfDay(ms: number): Date {
  const d = new Date(ms)
  return new Date(d.getFullYear(), d.getMonth(), d.getDate())
}

export function dayLabel(day: Date, now: Date, locale: string, t: Translate): string {
  const today = startOfDay(now.getTime())
  const yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1)
  if (day.getTime() === today.getTime()) return t('newsToday')
  if (day.getTime() === yesterday.getTime()) return t('newsYesterday')
  return formatChangeDate(day.getTime(), locale, day.getFullYear() !== today.getFullYear())
}

export function newsSource(url: string): string {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return ''
  }
}

export function buildNewsFeed(changes: RouteChange[], news: NewsItem[], now: Date, locale: string, t: Translate): NewsEntry[] {
  return [
    ...changes.map((change): NewsEntry => ({
      kind: 'change',
      key: `change-${change.id}`,
      at: change.detected_at,
      date: dayLabel(startOfDay(change.detected_at), now, locale, t),
      change,
    })),
    ...news.map((item): NewsEntry => {
      const day = parseNewsDate(item.date)
      return {
        kind: 'news',
        key: `news-${item.url}`,
        at: day?.getTime() ?? 0,
        date: day ? dayLabel(day, now, locale, t) : item.date,
        item,
      }
    }),
  ].sort((a, b) => b.at - a.at)
}
