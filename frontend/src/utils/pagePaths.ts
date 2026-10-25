import type {RouteLocationNormalized, RouteLocationRaw} from 'vue-router'
import {cityPath} from '@/utils/city.ts'
import {loadedNetwork, network, stationOf} from '@/utils/network.ts'
import type {RouteDirection} from '@/types/tranzy.ts'

export type Direction = RouteDirection | 'auto'

// Same paths as backend/pages/addresses.go.
export function stopPath(stopId: number | string): string {
  const slug = network.value?.stopById.get(Number(stopId))?.slug
  return cityPath(slug ? `/statia/${slug}` : `/stop/${stopId}`)
}

export const stopLocation = (stopId: number | string): RouteLocationRaw => ({path: stopPath(stopId), state: {platform: Number(stopId)}})

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

export function lineProps(to: RouteLocationNormalized) {
  const direction = to.params.direction
  return {
    routeId: String(network.value?.routeBySlug.get(String(to.params.line)) ?? ''),
    direction: direction === 'intors' ? '1' : direction === 'auto' ? 'auto' : '0',
  }
}

export async function resolvePagePath(to: RouteLocationNormalized): Promise<RouteLocationRaw | true> {
  if (to.name !== 'stop' && to.name !== 'route' && to.name !== 'stop-id' && to.name !== 'route-id') return true
  await loadedNetwork()
  const named = (path: string, state?: {platform: number}) =>
    /\/(statia|linia)\//.test(path) ? {path, query: to.query, hash: to.hash, replace: true, state} : true
  if (to.name === 'stop-id') return named(stopPath(String(to.params.stopId)), {platform: Number(to.params.stopId)})
  if (to.name === 'route-id') return named(linePath(String(to.params.routeId), String(to.params.direction) as Direction))
  return true
}
