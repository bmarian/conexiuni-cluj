// Turning a timetable into "how long until the next one" is the half of an arrival
// list that live tracking cannot do for itself, and it is where the ghost entries came
// from. Both the route stop list and the stop departure cards used
//
//   ((arrivalMinutes - nowMinutes) + 1440) % 1440
//
// which cannot tell a run that has already gone from one that is due this very minute:
// a run that passed a minute ago read as 1439 and was silently dropped by the horizon
// filter, while a run passing exactly now read as 0 and advertised itself as "now".
// A bus normally runs a few minutes off its timetable, so there is always some stop
// just behind the vehicle whose slot is sitting on zero - that stop showed a "now" the
// bus had already left, every stop behind it had lost its slot to the wrap and jumped a
// full headway, and every stop ahead of it was driven by live tracking. Reading down
// the column gave 24m, now, now, 2m.

/**
 * Minutes from `nowMinutes` until `absMinutes`, both in minutes past midnight.
 * Signed: negative means it has already happened. Differences beyond half a day are
 * read as crossing midnight rather than as a run most of a day away.
 */
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
  /** A run this many minutes late is still listed: the server learned how late runs are. */
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

/**
 * A run a couple of minutes late reads zero at every stop it is overdue at, so a stop
 * list said "now" several stops running for one bus. The first of consecutive zeros in
 * a column keeps "now" and the rest read "soon", as does a second zero within a row.
 */
export const markRepeatedNow = (rows: Arrival[][]): (Arrival & { soon: boolean })[][] =>
  rows.map((row, stop) => row.map((arrival, column) => ({
    ...arrival,
    soon: readsNow(arrival) && (readsNow(rows[stop - 1]?.[column]) || row.slice(0, column).some(readsNow)),
  })))
