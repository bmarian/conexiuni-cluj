// The cities' own time, which timetables, learned offsets and reminders are written in, wherever the phone is.
const CITY_ZONE = 'Europe/Bucharest'

const PARTS = new Intl.DateTimeFormat('en-GB', {
  timeZone: CITY_ZONE, weekday: 'short', year: 'numeric', month: '2-digit', day: '2-digit',
  hour: 'numeric', minute: 'numeric', second: 'numeric', hourCycle: 'h23',
})

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

export type CityTime = {
  // YYYY-MM-DD
  date: string
  // 0 is Sunday.
  weekday: number
  hour: number
  minute: number
  second: number
}

export function cityTime(at: Date | number): CityTime {
  const parts = PARTS.formatToParts(typeof at === 'number' ? new Date(at) : at)
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? ''
  return {
    date: `${get('year')}-${get('month')}-${get('day')}`,
    weekday: WEEKDAYS.indexOf(get('weekday')),
    hour: Number(get('hour')) % 24,
    minute: Number(get('minute')),
    second: Number(get('second')),
  }
}

export const cityDate = (at: Date | number): string => cityTime(at).date

// Minutes past the city's midnight, the seconds as a fraction.
export function cityMinutes(at: Date | number): number {
  const t = cityTime(at)
  return t.hour * 60 + t.minute + t.second / 60
}

// "HH:MM" on the city's clock.
export function cityClock(at: Date | number): string {
  const t = cityTime(at)
  return `${String(t.hour).padStart(2, '0')}:${String(t.minute).padStart(2, '0')}`
}

// "YYYY-MM-DDTHH:MM" on the city's clock, as a datetime-local input and the planner take it.
export const cityDateTime = (at: Date | number): string => `${cityDate(at)}T${cityClock(at)}`

function offsetMs(ms: number): number {
  const t = cityTime(ms)
  const [y, m, d] = t.date.split('-').map(Number)
  return Date.UTC(y!, m! - 1, d!, t.hour, t.minute) - Math.floor(ms / 60_000) * 60_000
}

// cityAt is the moment a date's clock in the city reads minutes past midnight.
export function cityAt(date: string, minutes: number): number {
  const [y, m, d] = date.split('-').map(Number)
  const local = Date.UTC(y!, m! - 1, d!) + minutes * 60_000
  const guess = local - offsetMs(local)
  return local - offsetMs(guess)
}
