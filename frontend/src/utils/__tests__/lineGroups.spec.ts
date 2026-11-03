import {describe, expect, it} from 'vitest'
import {NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import {divergingStops, groupKey, readLines, writeLines} from '@/utils/lineGroups.ts'

const stop = (stop_id: number, station_id = stop_id) => ({stop_id, stop_name: `S${stop_id}`, stop_lat: 46.77 + stop_id / 1000, stop_lon: 23.6, station_id})
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

// A rides 1→6; B and C join at 4 and split after 8; E rides 3, 4, 9 against A going back.
const index = new NetworkIndex({
  version: '',
  static_version: '',
  generated_at: '',
  stops: [...[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15].map((id) => stop(id)), stop(14, 4)],
  routes: [
    route(1, 'a', {'1_0': [6, 5, 4, 3, 2, 1], '1_1': [1, 2, 3, 4, 5, 6]}),
    route(2, 'b', {'2_1': [7, 4, 8, 9]}),
    route(3, 'c', {'3_1': [10, 4, 8, 11]}),
    route(4, 'd', {'4_0': [12, 13]}),
    route(5, 'e', {'5_0': [3, 4, 9]}),
    route(6, 'f', {'6_0': [14, 15]}),
  ],
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

describe('stations where lines split', () => {
  it('marks where lines riding together join or split, nothing where they cross or run the other way', () => {
    expect(
      divergingStops(index, [
        {routeId: 2, direction: '1'},
        {routeId: 3, direction: '1'},
      ]),
    ).toEqual([4, 8])
    expect(
      divergingStops(index, [
        {routeId: 1, direction: '1'},
        {routeId: 2, direction: '1'},
      ]),
    ).toEqual([])
    expect(
      divergingStops(index, [
        {routeId: 1, direction: '0'},
        {routeId: 5, direction: '0'},
      ]),
    ).toEqual([])
  })
})
