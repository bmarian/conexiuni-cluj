import type {Update} from '@/stores/routeUpdates'
import {formatChangeDate} from '@/utils/routeChanges'

type Translate = (key: string) => string

export type NewsEntry = Update & { key: string; date: string }

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

export function buildNewsFeed(updates: Update[], now: Date, locale: string, t: Translate): NewsEntry[] {
  return updates.map((update): NewsEntry => ({
    ...update,
    key: update.kind === 'change' ? `change-${update.change.id}` : `news-${update.news.url}`,
    date: dayLabel(startOfDay(update.at), now, locale, t),
  }))
}
