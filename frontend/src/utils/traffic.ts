import {computed, shallowRef} from 'vue'
import type {Paint} from '@/utils/lineSweep.ts'
import {decodePolyline} from '@/utils/network.ts'
import {locate, pathAlong} from '@/utils/vehicleMotion.ts'

export type TrafficStretch = {path: string; band: number}
export type Jam = {from_stop_id: number; to_stop_id: number; band: number; delay_sec: number}
export type Traffic = {stretches: TrafficStretch[]; jams: Jam[]}
export type TrafficByTrip = Record<string, Traffic>

const streams = shallowRef(new Map<object, TrafficByTrip>())

export const liveTraffic = computed<TrafficByTrip>(() => Object.assign({}, ...streams.value.values()))

export function publishTraffic(stream: object, traffic: TrafficByTrip | null) {
  const next = new Map(streams.value)
  if (traffic) next.set(stream, traffic)
  else next.delete(stream)
  streams.value = next
}

export function worstJam(traffic: Traffic | undefined, stopIds: number[], fromStopId?: number): Jam | undefined {
  const from = fromStopId === undefined ? 0 : stopIds.indexOf(fromStopId)
  if (!traffic || from < 0) return undefined
  return traffic.jams
    .filter((jam) => stopIds.indexOf(jam.from_stop_id) >= from)
    .reduce<Jam | undefined>((worst, jam) => (worst && worst.delay_sec >= jam.delay_sec ? worst : jam), undefined)
}

export const jamMinutes = (jam: Jam) => Math.max(1, Math.round(jam.delay_sec / 60))

export const TRAFFIC_COLORS = ['#34a853', '#fbbc04', '#ea4335', '#a50e0e'] as const

// The line painted by its stretches' bands, in its own colour where nothing is measured.
export function trafficPaint(stretches: TrafficStretch[], coords: [number, number][], color: string): Paint {
  const path = pathAlong(coords)
  const paint: Paint = [{at: 0, color}]
  let end = 0
  for (const stretch of stretches) {
    const points = decodePolyline(stretch.path)
    if (points.length < 2 || !path.length) continue
    const from = locate(path, {lat: points[0]![0], lng: points[0]![1]}, end)?.s
    const to = from === undefined ? undefined : locate(path, {lat: points.at(-1)![0], lng: points.at(-1)![1]}, from)?.s
    if (from === undefined || to === undefined || to <= from) continue
    if (from > end) paint.push({at: end / path.length, color})
    paint.push({at: from / path.length, color: TRAFFIC_COLORS[stretch.band] ?? color})
    end = to
  }
  if (end < path.length) paint.push({at: end / path.length, color})
  const out: Paint = []
  for (const stop of paint) {
    if (out.at(-1)?.at === stop.at) out.pop()
    if (out.at(-1)?.color !== stop.color) out.push(stop)
  }
  return out
}
