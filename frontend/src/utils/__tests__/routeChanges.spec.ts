import {describe, expect, it} from 'vitest'
import {createI18n} from 'vue-i18n'
import type {RouteChange, RouteChangeDay} from '@/stores/routeUpdates'
import {changeKindLabels, changeKinds, changeRows, changeSummary, groupByDirection, itemEntries, itemLabel} from '@/utils/routeChanges.ts'
import en from '@/locales/en.json'
import ro from '@/locales/ro.json'

const t = (key: string, named?: Record<string, unknown>) => (named ? `${key} ${JSON.stringify(named)}` : key)

// Line 1 after the September 2026 reissue, trimmed from what the backend recorded against ctpcj.ro.
const line1: RouteChange = {
  id: 7,
  route_short_name: '1',
  route_id: 1,
  route_color: '#e11d48',
  source: 'timetable',
  detected_at: 0,
  changes: [
    {kind: 'earlier', direction: '1', toward: 'Disp. Clabucet', days: ['weekdays'], shifts: [{from: '15:35', to: '15:30'}, {from: '15:52', to: '15:51'}]},
    {kind: 'later', direction: '0', toward: 'P-ta 1 Mai', days: ['weekdays'], shifts: [{from: '07:00', to: '07:01'}, {from: '07:35', to: '07:37'}]},
    {kind: 'trips_added', direction: '0', toward: 'P-ta 1 Mai', days: ['saturday', 'sunday'], all: true, times: ['05:50', '22:40']},
  ],
}

describe('route change text', () => {
  it('lists kinds in a stable order regardless of payload order', () => {
    expect(changeKinds(line1)).toEqual(['later', 'earlier', 'trips_added'])
  })

  it('uses one label for earlier and later departures in the news list', () => {
    expect(changeKindLabels(line1, t)).toEqual(['routeChangeKindRetimed', 'routeChangeKindTripsAdded'])
  })

  it('merges earlier and later departures into one row in timetable order', () => {
    const rows = changeRows([
      {kind: 'later', direction: '0', shifts: [{from: '07:00', to: '07:01'}, {from: '23:58', to: '00:03'}]},
      {kind: 'trips_removed', direction: '0', times: ['09:30']},
      {kind: 'earlier', direction: '0', shifts: [{from: '00:20', to: '00:15'}, {from: '07:35', to: '07:30'}]},
      {kind: 'trips_added', direction: '0', times: ['10:00']},
    ], t)
    expect(rows.map((r) => r.kind)).toEqual(['retimed', 'trips_added', 'trips_removed'])
    expect(rows[0]!.label).toBe('routeChangeRetimed {"n":4}')
    expect(rows[0]!.entries.map((e) => `${e.old}>${e.text}`)).toEqual(['07:00>07:01', '07:35>07:30', '23:58>00:03', '00:20>00:15'])
    expect(rows[2]!.entries).toEqual([{text: '09:30', struck: true}])
  })

  it('describes a newly served day by its span instead of every trip', () => {
    const item = line1.changes[2]!
    expect(itemLabel(item, t)).toBe('routeChangeNowRuns {"first":"05:50","last":"22:40"}')
    expect(itemEntries(item)).toEqual([])
  })

  it('groups items by direction in first-seen order', () => {
    const groups = groupByDirection(line1)
    expect(groups.map((g) => [g.direction, g.toward, g.items.length])).toEqual([
      ['1', 'Disp. Clabucet', 1],
      ['0', 'P-ta 1 Mai', 2],
    ])
  })
})

describe('route change summary', () => {
  const i18n = createI18n({legacy: false, locale: 'en', messages: {en, ro}})
  const say = (lang: 'en' | 'ro', change: RouteChange) => {
    i18n.global.locale.value = lang
    return changeSummary(change, (key, named) => (named ? i18n.global.t(key, named) : i18n.global.t(key)))
  }
  const timetable = (...days: RouteChangeDay[][]): RouteChange => ({
    ...line1,
    changes: days.map((d) => ({kind: 'later' as const, direction: '0' as const, days: d})),
  })
  const stops = (added: string[], removed: string[]): RouteChange => ({
    ...line1,
    source: 'stops',
    changes: [
      ...(added.length ? [{kind: 'stops_added' as const, direction: '0' as const, stops: added}] : []),
      ...(removed.length ? [{kind: 'stops_removed' as const, direction: '1' as const, stops: removed}] : []),
    ],
  })

  it.each([
    [[['sunday']], 'There are changes to the Sunday timetable.', 'Sunt schimbări la orarul de duminică.'],
    [[['weekdays']], 'There are changes to the weekday timetable.', 'Sunt schimbări la orarul din timpul săptămânii.'],
    [[['saturday', 'sunday']], 'There are changes to the Saturday and Sunday timetables.', 'Sunt schimbări la orarul de sâmbătă și de duminică.'],
    [[['sunday'], ['weekdays']], 'There are changes to the weekday and Sunday timetables.', 'Sunt schimbări la orarul din timpul săptămânii și de duminică.'],
    [[['weekdays'], ['saturday', 'sunday'], ['weekdays']],
      'There are changes to the weekday, Saturday and Sunday timetables.',
      'Sunt schimbări la orarul din timpul săptămânii, de sâmbătă și de duminică.'],
  ] as [RouteChangeDay[][], string, string][])('names the changed days %j', (days, enText, roText) => {
    expect(say('en', timetable(...days))).toBe(enText)
    expect(say('ro', timetable(...days))).toBe(roText)
  })

  it('counts a stop dropped in both directions once', () => {
    expect(say('en', stops([], ['Piața Mărăști', 'Piața Mărăști']))).toBe('A stop was removed from the route.')
    expect(say('ro', stops([], ['Piața Mărăști', 'Piața Mărăști']))).toBe('O stație a fost scoasă de pe traseu.')
  })

  it('covers added, removed and mixed stop changes', () => {
    expect(say('en', stops(['A'], []))).toBe('A new stop was added to the route.')
    expect(say('ro', stops(['A', 'B'], []))).toBe('Au fost adăugate stații noi pe traseu.')
    expect(say('en', stops([], ['A', 'B']))).toBe('Some stops were removed from the route.')
    expect(say('ro', stops(['A'], ['B']))).toBe('Unele stații au fost adăugate pe traseu, iar altele au fost scoase.')
  })
})
