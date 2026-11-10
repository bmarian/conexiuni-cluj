import type {Shape} from '@/types/tranzy.ts'
import type {NetworkIndex} from '@/utils/network.ts'
import {type GroupLine, stretchOf} from '@/utils/lineGroups.ts'
import {stopsAlong} from '@/utils/stopListBuses.ts'
import {locate, type Path, pathOf, pointAt} from '@/utils/vehicleMotion.ts'

export type Span = {path: Path; start: number; end: number}

// Null when the whole line is followed or a stop can't be placed on the shape.
export function stretchSpan(index: NetworkIndex, line: GroupLine, shape: Shape[]): Span | null {
  const {stops, from, to} = stretchOf(index, line)
  const whole = from < 0 && to >= stops.length - 1
  const places = stops.flatMap((id) => index.stopById.get(id) ?? [])
  if (whole || !shape.length || places.length < stops.length) return null
  const path = pathOf(shape)
  const at = stopsAlong(path, places)
  const start = from < 0 ? 0 : at[from]
  const end = to >= stops.length - 1 ? path.length : at[to]
  return typeof start === 'number' && typeof end === 'number' && end > start ? {path, start, end} : null
}

export function stretchShape(shape: Shape[], {path, start, end}: Span): Shape[] {
  const edge = (s: number): Shape => {
    const {lat, lng} = pointAt(path, s)
    return {...shape[0]!, shape_pt_lat: lat, shape_pt_lon: lng}
  }
  const inside = shape.filter((_, i) => path.at[i]! > start && path.at[i]! < end)
  return [edge(start), ...inside, edge(end)].map((point, i) => ({...point, shape_pt_sequence: i}))
}

export function followedPart(index: NetworkIndex, line: GroupLine, shape: Shape[]): Shape[] | null {
  const span = stretchSpan(index, line, shape)
  return span && stretchShape(shape, span)
}

export function pastStretch({path, end}: Span, at: {latitude: number; longitude: number}): boolean {
  const place = locate(path, {lat: at.latitude, lng: at.longitude})
  return !!place && place.s > end
}
