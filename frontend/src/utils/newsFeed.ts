import type {RouteChange} from '@/stores/routeUpdates'
import {formatChangeDate} from '@/utils/routeChanges'

type Translate = (key: string) => string

export type NewsEntry = { key: string; at: number; date: string; change: RouteChange }

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

// Changes on the followed lines, newest first.
export function buildNewsFeed(changes: RouteChange[], now: Date, locale: string, t: Translate): NewsEntry[] {
  return changes.map((change): NewsEntry => ({
    key: `change-${change.id}`,
    at: change.detected_at,
    date: dayLabel(startOfDay(change.detected_at), now, locale, t),
    change,
  })).sort((a, b) => b.at - a.at)
}
