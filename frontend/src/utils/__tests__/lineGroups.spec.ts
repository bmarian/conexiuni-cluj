import {describe, expect, it} from 'vitest'
import {NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import {type Departure, groupKey, type LegLine, legsNear, meetingLines, planLegs, readLines, timeLegs, writeLines} from '@/utils/lineGroups.ts'

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

// A rides 1→6 through station 4; B and C leave 4 for 8; F leaves from 14, another platform of 4; D meets nothing.
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

const first = (stops: number[]) => stops[0]!
const at2 = (stops: number[]) => (stops.includes(2) ? 2 : stops[0]!)

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

describe('legs', () => {
  it('changes where the next line meets the one before, and takes lines leaving together as one leg', () => {
    const legs = planLegs(
      index,
      [
        {routeId: 1, direction: '1'},
        {routeId: 2, direction: '1'},
        {routeId: 3, direction: '1'},
      ],
      at2,
    )
    expect(legs).toEqual([
      {lines: [{routeId: 1, direction: '1', stopId: 2}]},
      {
        lines: [
          {routeId: 2, direction: '1', stopId: 4},
          {routeId: 3, direction: '1', stopId: 4},
        ],
        alight: [{routeId: 1, direction: '1', stopId: 4}],
      },
    ])
  })

  it('changes where a line riding along leaves the way, as the picker names it', () => {
    const legs = planLegs(
      index,
      [
        {routeId: 1, direction: '1'},
        {routeId: 5, direction: '0'},
      ],
      at2,
    )
    expect(legs[1]).toEqual({lines: [{routeId: 5, direction: '0', stopId: 4}], alight: [{routeId: 1, direction: '1', stopId: 4}]})
  })

  it('keeps the change when you are past it, boarding at your nearest stop before it', () => {
    const past = {latitude: 46.775, longitude: 23.6}
    const legs = legsNear(
      index,
      [
        {routeId: 1, direction: '1'},
        {routeId: 2, direction: '1'},
      ],
      past,
    )
    expect(legs).toEqual([
      {lines: [{routeId: 1, direction: '1', stopId: 3}]},
      {lines: [{routeId: 2, direction: '1', stopId: 4}], alight: [{routeId: 1, direction: '1', stopId: 4}]},
    ])
  })

  it('walks to another platform of the same station', () => {
    const legs = planLegs(
      index,
      [
        {routeId: 1, direction: '1'},
        {routeId: 6, direction: '0'},
      ],
      at2,
    )
    expect(legs[1]).toEqual({lines: [{routeId: 6, direction: '0', stopId: 14}], alight: [{routeId: 1, direction: '1', stopId: 4}]})
  })

  it('never changes before the stop you board at, nor onto a line that ends there', () => {
    const legs = planLegs(
      index,
      [
        {routeId: 1, direction: '0'},
        {routeId: 2, direction: '1'},
      ],
      first,
    )
    expect(legs[1]?.alight).toEqual([{routeId: 1, direction: '0', stopId: 4}])
    expect(
      planLegs(
        index,
        [
          {routeId: 1, direction: '1'},
          {routeId: 2, direction: '1'},
        ],
        () => 5,
      )[1]?.alight,
    ).toBeUndefined()
  })

  it('keeps a line through the same stop going elsewhere, and one that never meets, on their own', () => {
    const legs = planLegs(
      index,
      [
        {routeId: 2, direction: '1'},
        {routeId: 5, direction: '0'},
        {routeId: 4, direction: '0'},
      ],
      (stops) => (stops.includes(4) ? 4 : stops[0]!),
    )
    expect(legs).toEqual([
      {lines: [{routeId: 2, direction: '1', stopId: 4}]},
      {lines: [{routeId: 5, direction: '0', stopId: 4}]},
      {lines: [{routeId: 4, direction: '0', stopId: 12}]},
    ])
  })
})

describe('leg times', () => {
  const min = 60_000
  const legs = planLegs(
    index,
    [
      {routeId: 1, direction: '1'},
      {routeId: 2, direction: '1'},
      {routeId: 3, direction: '1'},
    ],
    at2,
  )
  const buses: Record<string, Departure[]> = {
    '1@2': [
      {routeId: 1, at: 3 * min, live: true, vehicleId: 7},
      {routeId: 1, at: 14 * min, live: false},
    ],
    '1@4': [
      {routeId: 1, at: 2 * min, live: true, vehicleId: 6},
      {routeId: 1, at: 11 * min, live: true, vehicleId: 7},
      {routeId: 1, at: 22 * min, live: false},
    ],
    '2@4': [
      {routeId: 2, at: 13 * min, live: true},
      {routeId: 2, at: 28 * min, live: false},
    ],
    '3@4': [
      {routeId: 3, at: 6 * min, live: true},
      {routeId: 3, at: 10 * min, live: false},
      {routeId: 3, at: 21 * min, live: false},
      {routeId: 3, at: 35 * min, live: false},
    ],
  }
  const at = (line: LegLine) => buses[`${line.routeId}@${line.stopId}`] ?? []

  it('counts the change from the bus you take, found by its vehicle, and strikes the last one you miss', () => {
    const [start, next] = timeLegs(legs, at, at, () => 0, 3)
    expect(start?.caught?.at).toBe(3 * min)
    expect(next?.arrival).toBe(11 * min)
    expect(next?.missed?.at).toBe(10 * min)
    expect(next?.caught?.at).toBe(13 * min)
    expect(next?.shown.map((d) => d.at)).toEqual([13, 21, 28].map((m) => m * min))
  })

  it('takes the next bus to arrive when the vehicle is not known, and adds the walk', () => {
    const unknown = (line: LegLine) => at(line).map((d) => ({...d, vehicleId: undefined}))
    const [, next] = timeLegs(legs, unknown, unknown, () => 3 * min, 3)
    expect(next?.arrival).toBe(11 * min)
    expect(next?.caught?.at).toBe(21 * min)
  })

  it('shows no change once nothing more reaches it', () => {
    const none = (line: LegLine) => (line.stopId === 4 && line.routeId === 1 ? [] : at(line))
    const [, next] = timeLegs(legs, none, none, () => 0, 3)
    expect(next?.arrival).toBeUndefined()
    expect(next?.caught).toBeUndefined()
  })
})

describe('lines meeting a line', () => {
  it('lists each line once, where it leaves the way after boarding, leaving out lines through the boarding stop', () => {
    expect(meetingLines(index, {routeId: 1, direction: '1', stopId: 2})).toEqual([
      {routeId: 2, direction: '1', stopId: 4, rode: 2},
      {routeId: 3, direction: '1', stopId: 4, rode: 2},
      {routeId: 5, direction: '0', stopId: 4, rode: 2},
      {routeId: 6, direction: '0', stopId: 14, rode: 2},
    ])
    expect(meetingLines(index, {routeId: 1, direction: '1', stopId: 3}).map((l) => l.routeId)).toEqual([2, 3, 6])
  })
})
