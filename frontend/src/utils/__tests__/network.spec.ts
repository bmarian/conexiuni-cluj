import {describe, expect, it} from 'vitest'
import fixture from './fixtures/network_fixture.json'
import {buildHourlyOffsets, buildStopInfo, clujClock, decodePolyline, NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import type {StopInfo} from '@/types/tranzy.ts'

const index = new NetworkIndex(fixture.bundle as unknown as NetworkBundle)

function comparable(info: StopInfo): StopInfo {
  const lines = new Set(info.shapes_info.map((s) => s.route_id))
  const onLines = (ids: string[]) => ids.filter((id) => lines.has(Number(id.split('_')[0]))).sort()
  return {
    ...info,
    outgoing_trip_ids: onLines(info.outgoing_trip_ids ?? []),
    incoming_trip_ids: onLines(info.incoming_trip_ids ?? []),
    shapes_info: info.shapes_info.map((s) => ({...s, stop_time: s.stop_time?.map(({offset_confidence: _, ...st}) => st)})),
  }
}

describe('network bundle', () => {
  it('builds the stop exactly as stop_info answered', () => {
    const built = buildStopInfo(index, fixture.stop_id, new Date(fixture.captured_at_ms))
    expect(built).not.toBeNull()
    expect(comparable(built!)).toEqual(comparable(fixture.stop_info as unknown as StopInfo))
  })

  it('builds the hourly offsets of every kind of day', () => {
    const route = index.routeByShortName.get(fixture.route_short_name)!
    for (const dayType of ['weekday', 'saturday', 'sunday'] as const) {
      expect(buildHourlyOffsets(index, route, dayType)).toEqual(fixture.hourly[dayType])
    }
  })

  it('knows nothing of a stop it does not have', () => {
    expect(buildStopInfo(index, -1)).toBeNull()
  })

  it('decodes a shape to the same points', () => {
    const points = decodePolyline(fixture.shape.polyline)
    expect(points.length).toBe(fixture.shape.points.length)
    points.forEach(([lat, lon], i) => {
      expect(Math.abs(lat - fixture.shape.points[i]![0]!)).toBeLessThan(6e-7)
      expect(Math.abs(lon - fixture.shape.points[i]![1]!)).toBeLessThan(6e-7)
    })
  })

  it('reads the day and hour in Cluj, wherever the browser is', () => {
    expect(clujClock(new Date('2026-10-10T21:30:00Z'))).toEqual({dayType: 'sunday', hour: 0})
    expect(clujClock(new Date('2026-10-09T06:00:00Z'))).toEqual({dayType: 'weekday', hour: 9})
  })
})
