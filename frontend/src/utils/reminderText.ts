import {translate as t} from '@/i18n.ts'
import {addDays, clujDate, formatSlot} from '@/utils/reminders.ts'
import {clujMinutes} from '@/utils/follow.ts'

const weekday = (locale: string, iso: number, style: 'short' | 'narrow') =>
  // 2024-01-01 was a Monday.
  new Intl.DateTimeFormat(locale === 'en' ? 'en-GB' : 'ro-RO', {weekday: style, timeZone: 'UTC'})
    .format(new Date(Date.UTC(2024, 0, iso)))

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

export const dayLetter = (locale: string, iso: number) => capitalize(weekday(locale, iso, 'narrow'))

export function reminderWhen(atMs: number, locale: string, nowMs = Date.now()): string {
  const clock = formatSlot(clujMinutes(atMs))
  const day = clujDate(atMs)
  const today = clujDate(nowMs)
  if (day === today) return t('remindToday', {time: clock})
  if (day === addDays(today, 1)) return t('remindTomorrow', {time: clock})
  const iso = ((new Date(`${day}T12:00:00Z`).getUTCDay() + 6) % 7) + 1
  return `${capitalize(weekday(locale, iso, 'short').replace('.', ''))} ${clock}`
}

export function reminderRepeat(days: number[], locale: string): string | null {
  const sorted = [...days].sort((a, b) => a - b).join(',')
  if (!sorted) return null
  if (sorted === '1,2,3,4,5,6,7') return t('remindEveryDay')
  if (sorted === '1,2,3,4,5') return t('weekdays')
  if (sorted === '6,7') return t('remindWeekends')
  return [...days].sort((a, b) => a - b).map((d) => capitalize(weekday(locale, d, 'short').replace('.', ''))).join(', ')
}
