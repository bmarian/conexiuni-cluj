export const num = (n: number) => n.toLocaleString('en-GB')
export const pct = (share: number) => `${Math.round(share * 100)}%`

export function dur(sec: number) {
  sec = Math.round(Math.abs(sec))
  if (sec < 60) return `${sec} s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  if (h < 48) return min % 60 ? `${h} h ${min % 60} min` : `${h} h`
  return `${Math.floor(h / 24)} days`
}

export function ago(at: string | number | null | undefined, now: number) {
  if (at == null || at === '') return 'never'
  const t = typeof at === 'number' ? at : new Date(at).getTime()
  if (!Number.isFinite(t)) return 'never'
  const sec = (now - t) / 1000
  if (sec < 5) return 'just now'
  return sec > 0 ? `${dur(sec)} ago` : `in ${dur(-sec)}`
}

export function bytes(n: number) {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / 1024 ** i
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export function clockTime(iso: string | null | undefined) {
  if (!iso) return ''
  return new Date(iso).toLocaleTimeString('en-GB', {hour: '2-digit', minute: '2-digit'})
}

export function dayName(date: string) {
  return new Date(`${date}T12:00:00`).toLocaleDateString('en-GB', {weekday: 'short', day: 'numeric', month: 'short'})
}

export function minSec(sec: number) {
  sec = Math.round(Math.abs(sec))
  return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`
}

export function came(sec: number) {
  if (Math.abs(sec) < 15) return 'on time'
  return `${minSec(sec)} ${sec > 0 ? 'late' : 'early'}`
}

export const platformLabels: Record<string, string> = {
  web: 'Web',
  android: 'Android',
  wear: 'Watch',
}

export const timetableLabels: Record<string, string> = {
  weekday: 'Weekday',
  saturday: 'Saturday',
  sunday: 'Sunday',
}
