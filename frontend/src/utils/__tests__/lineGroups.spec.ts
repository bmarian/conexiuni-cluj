import {describe, expect, it} from 'vitest'
import {NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import {boardingStop, groupKey, readLines, writeLines} from '@/utils/lineGroups.ts'

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
})
