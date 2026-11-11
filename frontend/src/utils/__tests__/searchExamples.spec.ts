import {describe, expect, it} from 'vitest'
import {NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import {searchExamples} from '@/utils/searchExamples.ts'

const stop = (stop_id: number) => ({stop_id, stop_name: `S${stop_id}`, stop_lat: 46.77, stop_lon: 23.6, station_id: stop_id})
const day = (departures: number) => ({
  service_name: '',
  service_start: '',
  entries: Array.from({length: departures}, () => ({departure_in: '', departure_out: ''})),
})
const route = (route_id: number, weekdayDepartures: number, stop_ids: number[]) => ({
  route_id,
  agency_id: 1,
  route_short_name: `L${route_id}`,
  route_long_name: '',
  route_type: 3,
  route_desc: '',
  route_color: '#1D71B8',
  timetable: {weekdays: day(weekdayDepartures), saturday: day(0), sunday: day(0)},
  offsets: {},
  trips: [{trip_id: `${route_id}_0`, stop_ids, stop_sequences: stop_ids.map((_, i) => i)}],
})

const index = new NetworkIndex({
  version: '',
  static_version: '',
  generated_at: '',
  stops: [1, 2, 3, 4].map(stop),
  routes: [route(1, 40, [1, 2]), route(2, 90, [2, 3]), route(3, 10, [2, 4])],
} as unknown as NetworkBundle)

const area = (name: string, stops: number, district = true) => ({slug: name, name, district, stops, lines: 1})

describe('search examples', () => {
  it('picks the busiest line, the station with most lines and the neighbourhood with most stops', () => {
    const examples = searchExamples(index, [area('Cluj-Napoca', 50, false), area('Centru', 8), area('Mănăștur', 34)])
    expect(examples.line?.route_short_name).toBe('L2')
    expect(examples.stop?.stop_name).toBe('S2')
    expect(examples.place).toBe('Mănăștur')
  })

  it('has no place without neighbourhoods', () => {
    expect(searchExamples(index, [area('Buzău', 20, false)]).place).toBeUndefined()
  })
})
