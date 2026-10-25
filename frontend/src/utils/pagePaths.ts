import type {RouteLocationNormalized, RouteLocationRaw} from 'vue-router'
import {cityPath} from '@/utils/city.ts'
import {loadedNetwork, network, stationOf} from '@/utils/network.ts'
import type {RouteDirection} from '@/types/tranzy.ts'

export type Direction = RouteDirection | 'auto'

const stationSlug = (stopId: number | string) => network.value?.stopById.get(Number(stopId))?.slug

// Same paths as backend/pages/addresses.go.
export function stopPath(stopId: number | string): string {
  const slug = stationSlug(stopId)
  return cityPath(slug ? `/statia/${slug}` : `/stop/${stopId}`)
}

export function stopsPath(stopIds: string[]): string {
  const slugs = stopIds.map(stationSlug)
  return cityPath(slugs.every(Boolean) ? `/statii/${slugs.join(',')}` : `/stops/${stopIds.join(',')}`)
}

export const stopLocation = (stopId: number | string): RouteLocationRaw => ({path: stopPath(stopId), state: {platform: Number(stopId)}})

export const areaPath = (slug: string) => cityPath(`/spre/${slug}`)

export function linePath(routeId: number | string, direction: Direction): string {
  const slug = network.value?.routeById.get(Number(routeId))?.slug
  if (!slug) return cityPath(`/route/${routeId}/${direction}`)
  return cityPath(`/linia/${slug}${direction === '1' ? '/intors' : direction === 'auto' ? '/auto' : ''}`)
}

export const lineCanonicalPath = (routeId: number | string, direction: Direction) =>
  network.value?.routeById.get(Number(routeId))?.slug ? linePath(routeId, '0') : linePath(routeId, direction)

export function stationProps(to: RouteLocationNormalized) {
  const index = network.value
  const station = index?.stationBySlug.get(String(to.params.station))
  const platform = index?.stopById.get(Number(history.state?.platform))
  return {stopId: String(platform && stationOf(platform) === station ? platform.stop_id : (station ?? ''))}
}

export const stationsProps = (to: RouteLocationNormalized) => ({
  stopIds: String(to.params.stations)
    .split(',')
    .map((slug) => network.value?.stationBySlug.get(slug) ?? '')
    .join(','),
})

export function lineProps(to: RouteLocationNormalized) {
  const direction = to.params.direction
  return {
    routeId: String(network.value?.routeBySlug.get(String(to.params.line)) ?? ''),
    direction: direction === 'intors' ? '1' : direction === 'auto' ? 'auto' : '0',
  }
}

const NAMED_PAGES = new Set(['stop', 'stop-id', 'stops', 'stops-id', 'route', 'route-id'])

export async function resolvePagePath(to: RouteLocationNormalized): Promise<RouteLocationRaw | true> {
  if (!NAMED_PAGES.has(String(to.name))) return true
  await loadedNetwork()
  const named = (path: string, state?: {platform: number}) =>
    /\/(statia|statii|linia)\//.test(path) ? {path, query: to.query, hash: to.hash, replace: true, state} : true
  if (to.name === 'stop-id') return named(stopPath(String(to.params.stopId)), {platform: Number(to.params.stopId)})
  if (to.name === 'stops-id') return named(stopsPath(String(to.params.stopIds).split(',')))
  if (to.name === 'route-id') return named(linePath(String(to.params.routeId), String(to.params.direction) as Direction))
  const stations = to.name === 'stops' ? String(to.params.stations).split(',') : []
  return stations.every((slug) => network.value?.stationBySlug.has(slug)) || named(stopsPath(stations))
}
