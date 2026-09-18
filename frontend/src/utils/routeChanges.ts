import type {RouteChange, RouteChangeDay, RouteChangeItem, RouteChangeKind} from '@/stores/routeUpdates'
import {timeStringToMinutes} from '@/utils/time'

type Translate = (key: string, named?: Record<string, unknown>) => string

const KIND_ORDER: RouteChangeKind[] = ['later', 'earlier', 'trips_added', 'trips_removed', 'stops_added', 'stops_removed']

const DAY_ORDER: RouteChangeDay[] = ['weekdays', 'saturday', 'sunday']

const DAY_PHRASE_KEYS: Record<RouteChangeDay, string> = {
  weekdays: 'routeChangeDayWeekdays',
  saturday: 'routeChangeDaySaturday',
  sunday: 'routeChangeDaySunday',
}

const KIND_LABEL_KEYS: Record<RouteChangeKind, string> = {
  later: 'routeChangeKindRetimed',
  earlier: 'routeChangeKindRetimed',
  trips_added: 'routeChangeKindTripsAdded',
  trips_removed: 'routeChangeKindTripsRemoved',
  stops_added: 'routeChangeKindStopsAdded',
  stops_removed: 'routeChangeKindStopsRemoved',
}

export function changeKinds(change: RouteChange): RouteChangeKind[] {
  const present = new Set(change.changes.map((c) => c.kind))
  return KIND_ORDER.filter((k) => present.has(k))
}

export function changeKindLabels(change: RouteChange, t: Translate): string[] {
  return [...new Set(changeKinds(change).map((k) => t(KIND_LABEL_KEYS[k])))]
}

export function changeTitle(change: RouteChange, t: Translate): string {
  return t(change.source === 'stops' ? 'routeChangeStops' : 'routeChangeTimetable')
}

export function joinList(items: string[], and: string): string {
  if (items.length <= 1) return items[0] ?? ''
  return `${items.slice(0, -1).join(', ')} ${and} ${items[items.length - 1]}`
}

export function changeSummary(change: RouteChange, t: Translate): string {
  if (change.source === 'stops') {
    const stopsOf = (kind: RouteChangeKind) =>
      new Set(change.changes.filter((c) => c.kind === kind).flatMap((c) => c.stops ?? []))
    const added = stopsOf('stops_added')
    const removed = stopsOf('stops_removed')
    if (added.size && removed.size) return t('routeChangeSummaryStopsBoth')
    if (added.size) return t(added.size === 1 ? 'routeChangeSummaryStopAdded' : 'routeChangeSummaryStopsAdded')
    return t(removed.size === 1 ? 'routeChangeSummaryStopRemoved' : 'routeChangeSummaryStopsRemoved')
  }

  const present = new Set(change.changes.flatMap((c) => c.days ?? []))
  const days = DAY_ORDER.filter((d) => present.has(d)).map((d) => t(DAY_PHRASE_KEYS[d]))
  if (!days.length) return t('routeChangeSummaryTimetableAny')
  return t(days.length === 1 ? 'routeChangeSummaryTimetableOne' : 'routeChangeSummaryTimetableMany', {
    days: joinList(days, t('routeChangeListAnd')),
  })
}

export interface DayGroup {
  key: string
  days: RouteChangeDay[]
  since?: string
  items: RouteChangeItem[]
}

export function groupByDays(items: RouteChangeItem[]): DayGroup[] {
  const groups: DayGroup[] = []
  for (const item of items) {
    const key = (item.days ?? []).join(',')
    let group = groups.find((g) => g.key === key)
    if (!group) {
      group = {key, days: item.days ?? [], since: item.since, items: []}
      groups.push(group)
    }
    group.items.push(item)
  }
  return groups
}

export function changeDirection(change: RouteChange): '0' | '1' {
  return change.changes[0]?.direction ?? '0'
}

export function formatChangeDate(ms: number, locale: string): string {
  return new Date(ms).toLocaleDateString(locale === 'en' ? 'en-GB' : 'ro-RO', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

export function daysLabel(days: RouteChangeDay[] | undefined, t: Translate): string {
  return (days ?? []).map((d) => t(d)).join(', ')
}

export function itemLabel(item: RouteChangeItem, t: Translate): string {
  const times = item.times ?? []
  switch (item.kind) {
    case 'later':
    case 'earlier':
      return t('routeChangeRetimed', {n: item.shifts?.length ?? 0})
    case 'trips_added':
      return item.all
        ? t('routeChangeNowRuns', {first: times[0] ?? '', last: times[times.length - 1] ?? ''})
        : t('routeChangeTripsAdded', {n: times.length})
    case 'trips_removed':
      return item.all ? t('routeChangeNoLongerRuns') : t('routeChangeTripsRemoved', {n: times.length})
    case 'stops_added':
      return t('routeChangeStopsAdded')
    case 'stops_removed':
      return t('routeChangeStopsRemoved')
  }
}

export function itemChips(item: RouteChangeItem): string[] {
  if (item.shifts?.length) return item.shifts.map((s) => `${s.from} → ${s.to}`)
  if (item.all) return []
  return item.times ?? item.stops ?? []
}

export type ChangeRowKind = 'retimed' | 'trips_added' | 'trips_removed' | 'stops_added' | 'stops_removed'

export interface ChangeRow {
  kind: ChangeRowKind
  label: string
  chips: string[]
}

const ROW_ORDER: ChangeRowKind[] = ['retimed', 'trips_added', 'trips_removed', 'stops_added', 'stops_removed']

// The service day starts around 04:00, so 00:30 belongs after 23:30.
function serviceMinutes(time: string): number {
  const m = timeStringToMinutes(time) ?? 0
  return m < 240 ? m + 1440 : m
}

export function changeRows(items: RouteChangeItem[], t: Translate): ChangeRow[] {
  const isShift = (item: RouteChangeItem) => item.kind === 'later' || item.kind === 'earlier'
  const shifts = items
    .filter(isShift)
    .flatMap((item) => item.shifts ?? [])
    .sort((a, b) => serviceMinutes(a.from) - serviceMinutes(b.from))

  const rows: ChangeRow[] = items
    .filter((item) => !isShift(item))
    .map((item) => ({kind: item.kind as ChangeRowKind, label: itemLabel(item, t), chips: itemChips(item)}))
  if (shifts.length) {
    rows.push({
      kind: 'retimed',
      label: t('routeChangeRetimed', {n: shifts.length}),
      chips: shifts.map((s) => `${s.from} → ${s.to}`),
    })
  }
  return rows.sort((a, b) => ROW_ORDER.indexOf(a.kind) - ROW_ORDER.indexOf(b.kind))
}

export interface DirectionGroup {
  direction: '0' | '1'
  toward: string
  items: RouteChangeItem[]
}

export function groupByDirection(change: RouteChange): DirectionGroup[] {
  const groups: DirectionGroup[] = []
  for (const item of change.changes) {
    let group = groups.find((g) => g.direction === item.direction)
    if (!group) {
      group = {direction: item.direction, toward: item.toward ?? '', items: []}
      groups.push(group)
    }
    group.items.push(item)
  }
  return groups
}
