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

// A scheduled run may be this many minutes late and still be listed: buses lose a few
// minutes in traffic all the time, and dropping the entry only moves the whole list on
// by a full headway. Past this the run is treated as gone rather than as still pending.
export const SCHEDULE_LATE_GRACE_MIN = 3

export type ScheduleQuery = {
  /** Departures from the terminus, in minutes past midnight. */
  departureMinutes: number[]
  /** Minutes this trip takes from the terminus to the stop being asked about. */
  offsetMinutes: number
  /** Now, in minutes past midnight. */
  nowMinutes: number
  /** Ignore anything further out than this. Defaults to no limit. */
  horizonMinutes?: number
}

/** Upcoming timetable arrivals at one stop, soonest first. May start slightly negative. */
export const scheduledArrivals = (query: ScheduleQuery): number[] => {
  const horizon = query.horizonMinutes ?? Number.POSITIVE_INFINITY
  return query.departureMinutes
    .map((departure) => minutesUntil(departure + query.offsetMinutes, query.nowMinutes))
    .filter((minutes) => minutes >= -SCHEDULE_LATE_GRACE_MIN && minutes < horizon)
    .sort((a, b) => a - b)
}

export type Arrival = { minutes: number; isLive: boolean }

export type MergeOptions = {
  /**
   * Live tracking has vehicles on this trip, so its silence about this stop is
   * evidence rather than an absence of data: whatever the timetable still has sitting
   * on zero here is the run that has gone past, not one arriving now.
   */
  tracked?: boolean
  limit?: number
}

/**
 * One list of arrivals from the two sources, reconciled on time rather than on
 * position. The tracked bus *is* one of the scheduled runs, so the slot it answers for
 * has to come out of the list - otherwise the same vehicle is listed twice, once
 * tracked and once from a timetable that disagrees with it by a few minutes, and the
 * columns stop meaning the same thing from one row to the next.
 */
export const mergeArrivals = (
  liveMinutes: number | null,
  scheduled: number[],
  options: MergeOptions = {},
): Arrival[] => {
  const limit = options.limit ?? 3

  if (liveMinutes === null) {
    const pending = options.tracked ? scheduled.filter((minutes) => minutes >= 1) : scheduled
    return pending.slice(0, limit).map((minutes) => ({minutes: Math.max(minutes, 0), isLive: false}))
  }

  const live = Math.max(liveMinutes, 0)
  // Everything at or before the live estimate belongs to the run being tracked, or to
  // one that is already gone. What is left are the runs that follow it.
  const later = scheduled.filter((minutes) => minutes > live + SCHEDULE_LATE_GRACE_MIN)
  return [
    {minutes: live, isLive: true},
    ...later.slice(0, Math.max(limit - 1, 0)).map((minutes) => ({minutes, isLive: false})),
  ]
}
