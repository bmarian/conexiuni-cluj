/** Signed minutes until absMinutes; wraps across midnight the short way. */
export const minutesUntil = (absMinutes: number, nowMinutes: number): number => {
  const diff = absMinutes - nowMinutes
  if (diff < -720) return diff + 1440
  if (diff > 720) return diff - 1440
  return diff
}

export type ScheduleQuery = {
  /** Departures from the terminus, in minutes past midnight. */
  departureMinutes: number[]
  /** Seconds this trip takes from the terminus to the stop being asked about. */
  offsetSeconds: number
  /** Now, in minutes past midnight, with the seconds. */
  nowMinutes: number
  /** Ignore anything further out than this. Defaults to no limit. */
  horizonMinutes?: number
  /** A run this many minutes late is still listed. */
  graceMinutes: number
}

/** Upcoming timetable arrivals at one stop as exact minutes, soonest first. May start slightly negative. */
export const scheduledArrivals = (query: ScheduleQuery): number[] => {
  const horizon = query.horizonMinutes ?? Number.POSITIVE_INFINITY
  return query.departureMinutes
    .map((departure) => minutesUntil(departure + query.offsetSeconds / 60, query.nowMinutes))
    .filter((minutes) => minutes >= -query.graceMinutes && minutes < horizon)
    .sort((a, b) => a - b)
}

export type Arrival = { minutes: number; isLive: boolean }

/** The next runs from the timetable alone, for a list the live feed is not reaching, as the minutes shown. */
export const nextArrivals = (scheduled: number[], limit: number): Arrival[] =>
  scheduled.slice(0, limit).map((minutes) => ({minutes: Math.max(Math.round(minutes), 0), isLive: false}))

const readsNow = (arrival: Arrival | undefined): boolean => arrival !== undefined && Math.round(arrival.minutes) === 0

/** Only the first of consecutive zeros, down a column or along a row, reads "now"; the rest "soon". */
export const markRepeatedNow = (rows: Arrival[][]): (Arrival & { soon: boolean })[][] =>
  rows.map((row, stop) => row.map((arrival, column) => ({
    ...arrival,
    soon: readsNow(arrival) && (readsNow(rows[stop - 1]?.[column]) || row.slice(0, column).some(readsNow)),
  })))
