import {dateLocale} from '@/utils/text.ts'

const WEEK = 7

type Translate = (key: string) => string

function addDays(date: string, days: number): string {
  const [y, m, d] = date.split('-').map(Number)
  return new Date(Date.UTC(y!, m! - 1, d! + days)).toISOString().slice(0, 10)
}

export function planDays(today: string, chosen: string): string[] {
  const days = Array.from({length: WEEK}, (_, i) => addDays(today, i))
  return chosen && !days.includes(chosen) ? [...days, chosen] : days
}

export function planDayLabel(date: string, today: string, locale: string, t: Translate): string {
  if (date === today) return t('planToday')
  if (date === addDays(today, 1)) return t('planTomorrow')
  const label = new Date(`${date}T12:00:00Z`).toLocaleDateString(dateLocale(locale), {
    weekday: 'long',
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  })
  return label.charAt(0).toUpperCase() + label.slice(1)
}
