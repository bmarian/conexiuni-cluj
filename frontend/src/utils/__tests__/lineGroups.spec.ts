import {describe, expect, it} from 'vitest'
import {NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import type {Shape} from '@/types/tranzy.ts'
import {
  boardingStop,
  flipLine,
  groupKey,
  legsKey,
  legsOf,
  nearerLeg,
  pickStop,
  readLines,
  stopRows,
  stretchNames,
  withStops,
  writeLines,
} from '@/utils/lineGroups.ts'
import {pastStretch, stretchShape, stretchSpan} from '@/utils/lineStretch.ts'

const stop = (stop_id: number) => ({stop_id, stop_name: `S${stop_id}`, stop_lat: 46.77 + stop_id / 1000, stop_lon: 23.6, station_id: stop_id})
const route = (route_id: number, slug: string, trips: Record<string, number[]>) => ({
  route_id,
  agency_id: 1,
  route_short_name: slug.toUpperCase(),
  route_long_name: `${slug} start - ${slug} end`,
  route_type: 3,
  route_desc: '',
  route_color: '#1D71B8',
  slug,
  timetable: null,
  offsets: {},
  trips: Object.entries(trips).map(([trip_id, stop_ids]) => ({trip_id, stop_ids, stop_sequences: stop_ids.map((_, i) => i)})),
})

const index = new NetworkIndex({
  version: '',
  static_version: '',
  generated_at: '',
  stops: [1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 13].map((id) => stop(id)),
  routes: [route(1, 'a', {'1_0': [6, 5, 4, 3, 2, 1], '1_1': [1, 2, 3, 4, 5, 6]}), route(2, 'b', {'2_1': [7, 4, 8, 9]}), route(4, 'd', {'4_0': [12, 13]})],
} as unknown as NetworkBundle)

describe('line group addresses', () => {
  it('reads back what it writes, once per line', () => {
    const lines = [
      {routeId: 1, direction: '1' as const},
      {routeId: 2, direction: '1' as const},
      {routeId: 4, direction: '0' as const},
    ]
    expect(writeLines(index, lines)).toBe('a.intors,b.intors,d')
    expect(readLines(index, 'a.intors,b.intors,d,a,zz')).toEqual(lines)
    expect(groupKey(lines)).toBe('1/1,2/1,4/0')
  })
})

describe('boarding stop', () => {
  const near = (stopId: number) => ({latitude: 46.77 + stopId / 1000, longitude: 23.6})

  it('boards at the stop nearest you, never at the last one, and at the first without a location', () => {
    const a = {routeId: 1, direction: '1' as const}
    expect(boardingStop(index, a, near(3))).toBe(3)
    expect(boardingStop(index, a, near(6))).toBe(5)
    expect(boardingStop(index, a, null)).toBe(1)
  })

  it('boards at the chosen stop, and without one before the get-off stop', () => {
    const a = {routeId: 1, direction: '1' as const}
    expect(boardingStop(index, withStops(a, 4, undefined), near(1))).toBe(4)
    expect(boardingStop(index, withStops(a, undefined, 3), near(6))).toBe(2)
  })
})

describe('round trips', () => {
  const there = [withStops({routeId: 1, direction: '1' as const}, 1, 3)]
  const back = [withStops({routeId: 1, direction: '0' as const}, 3, 1)]
  const at = (stopId: number) => ({latitude: 46.77 + stopId / 1000, longitude: 23.6})

  it('keeps a group without a way back one-way', () => {
    expect(legsOf(there, [])).toEqual({lines: there})
    expect(legsKey(legsOf(there, back))).not.toBe(legsKey({lines: there}))
  })

  it('opens on the leg you board nearest to, the way there on a tie', () => {
    const legs = legsOf(there, back)
    expect(nearerLeg(index, legs, at(1))).toBe('dus')
    expect(nearerLeg(index, legs, at(3))).toBe('intors')
    expect(nearerLeg(index, legsOf(there, there), at(3))).toBe('dus')
    expect(nearerLeg(index, {lines: there}, at(3))).toBe('dus')
  })
})

describe('followed stops', () => {
  const a = {routeId: 1, direction: '1' as const}
  const b = {routeId: 2, direction: '1' as const}

  it('reads back the stops a line is followed between', () => {
    const lines = [withStops(a, 2, 5), withStops(b, undefined, 8), {routeId: 4, direction: '0' as const}]
    expect(writeLines(index, lines)).toBe('a.intors:2:5,b.intors::8,d')
    expect(readLines(index, 'a.intors:2:5,b.intors::8,d')).toEqual(lines)
    expect(groupKey(lines)).toBe('1/1/2/5,2/1//8,4/0')
    expect(stretchNames(index, lines[0]!)).toEqual({from: 'S2', to: 'S5'})
  })

  it('drops a stop the line does not serve, one out of order, the last to board at and the first to get off at', () => {
    expect(readLines(index, 'a.intors:7:5')).toEqual([withStops(a, undefined, 5)])
    expect(readLines(index, 'a.intors:5:2')).toEqual([withStops(a, 5, undefined)])
    expect(readLines(index, 'a.intors:6')).toEqual([a])
    expect(readLines(index, 'a.intors::1')).toEqual([a])
  })

  it('flips between the same stations, and to the whole line where there is no way back', () => {
    expect(flipLine(index, withStops(a, 2, 5))).toEqual({routeId: 1, direction: '0', from: 5, to: 2})
    expect(flipLine(index, withStops(b, 4, 8))).toEqual({routeId: 2, direction: '0'})
  })

  it('takes a tap for the end being chosen', () => {
    const boarded = pickStop(index, a, 'from', 3)
    expect(boarded).toEqual({line: withStops(a, 3, undefined), next: 'to'})
    expect(pickStop(index, boarded.line, 'to', 5)).toEqual({line: withStops(a, 3, 5), next: 'to'})
    expect(pickStop(index, withStops(a, 3, 5), 'to', 2)).toEqual({line: withStops(a, 2, undefined), next: 'to'})
    expect(pickStop(index, withStops(a, 3, 5), 'to', 6)).toEqual({line: withStops(a, 3, undefined), next: 'to'})
    expect(pickStop(index, withStops(a, 3, 5), 'from', 3)).toEqual({line: withStops(a, undefined, 5), next: 'from'})
    expect(pickStop(index, withStops(a, 3, 5), 'from', 5)).toEqual({line: withStops(a, 5, undefined), next: 'to'})
    expect(pickStop(index, a, 'from', 6)).toEqual({line: a, next: 'from'})
    expect(pickStop(index, a, 'to', 1)).toEqual({line: a, next: 'to'})
  })

  it('paints the list between the two stops and dims the rest', () => {
    const rows = stopRows(index, withStops(a, 2, 4), undefined)
    expect(rows.map((row) => row.role)).toEqual(['pass', 'board', 'pass', 'leave', 'pass', 'pass'])
    expect(rows.map((row) => row.dimmed)).toEqual([true, false, false, false, true, true])
    expect(rows[1]!.painted).toEqual({top: false, bottom: true})
    expect(rows[3]!.painted).toEqual({top: true, bottom: false})
    expect(stopRows(index, a, 3).map((row) => row.role)).toEqual(['pass', 'pass', 'near', 'pass', 'pass', 'pass'])
  })
})

describe('followed part of the shape', () => {
  const a = {routeId: 1, direction: '1' as const}
  const point = (id: number, i: number): Shape => ({
    shape_id: '1_1',
    shape_pt_lat: 46.77 + id / 1000,
    shape_pt_lon: 23.6,
    shape_pt_sequence: i,
    shape_dist_traveled: -1,
  })
  const shape = [1, 2, 3, 4, 5, 6].map(point)

  it('has no part to cut for a line followed whole', () => {
    expect(stretchSpan(index, a, shape)).toBeNull()
  })

  it('cuts the shape at the two stops and tells the buses past the second', () => {
    const span = stretchSpan(index, withStops(a, 2, 5), shape)!
    const lats = stretchShape(shape, span).map((p) => Number(p.shape_pt_lat.toFixed(3)))
    expect(lats).toEqual([46.772, 46.773, 46.774, 46.775])
    expect(pastStretch(span, {latitude: 46.776, longitude: 23.6})).toBe(true)
    expect(pastStretch(span, {latitude: 46.771, longitude: 23.6})).toBe(false)
  })
})
