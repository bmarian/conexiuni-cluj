import {afterEach, describe, expect, it, vi} from 'vitest'
import type {RouteLocationNormalized} from 'vue-router'
import {network, NetworkIndex, type NetworkBundle} from '@/utils/network.ts'
import {currentCity} from '@/utils/city.ts'
import {lineCanonicalPath, lineProps, linePath, stationProps, stationsProps, stopPath, stopsPath} from '@/utils/pagePaths.ts'

const stop = (stop_id: number, station_id: number, slug?: string) => ({stop_id, station_id, slug, stop_name: 'Piața Unirii'})

const bundle = {
  routes: [
    {route_id: 7, route_short_name: '24B', slug: '24b', trips: []},
    {route_id: 8, route_short_name: '25', trips: []},
  ],
  stops: [stop(10, 10, 'piata-unirii'), stop(11, 10, 'piata-unirii'), stop(12, 12), stop(13, 13, 'gara')],
} as unknown as NetworkBundle

const at = (params: Record<string, string>) => ({params}) as unknown as RouteLocationNormalized

const pickedFrom = (platform?: number) => vi.stubGlobal('history', {state: platform ? {platform} : null})

afterEach(() => {
  network.value = null
  vi.unstubAllGlobals()
})

describe('page paths', () => {
  it('names stations and lines once the network does', () => {
    currentCity.value = 'cluj'
    network.value = new NetworkIndex(bundle)
    expect(stopPath(11)).toBe('/cluj/statia/piata-unirii')
    expect(linePath(7, '0')).toBe('/cluj/linia/24b')
    expect(linePath(7, '1')).toBe('/cluj/linia/24b/intors')
    expect(lineCanonicalPath(7, '1')).toBe('/cluj/linia/24b')
  })

  it('keeps the id paths for what has no address', () => {
    currentCity.value = 'cluj'
    expect(stopPath(11)).toBe('/cluj/stop/11')
    network.value = new NetworkIndex(bundle)
    expect(stopPath(12)).toBe('/cluj/stop/12')
    expect(linePath(8, '1')).toBe('/cluj/route/8/1')
    expect(lineCanonicalPath(8, '1')).toBe('/cluj/route/8/1')
  })

  it('opens a station on the platform it was picked from', () => {
    network.value = new NetworkIndex(bundle)
    pickedFrom()
    expect(stationProps(at({station: 'piata-unirii'}))).toEqual({stopId: '10'})
    pickedFrom(11)
    expect(stationProps(at({station: 'piata-unirii'}))).toEqual({stopId: '11'})
    pickedFrom(12)
    expect(stationProps(at({station: 'piata-unirii'}))).toEqual({stopId: '10'})
  })

  it('names several stations in one address', () => {
    currentCity.value = 'cluj'
    network.value = new NetworkIndex(bundle)
    expect(stopsPath(['10', '13'])).toBe('/cluj/statii/piata-unirii,gara')
    expect(stopsPath(['10', '12'])).toBe('/cluj/stops/10,12')
    expect(stationsProps(at({stations: 'piata-unirii,gara,nope'}))).toEqual({stopIds: '10,13,'})
  })

  it('reads the direction from the address', () => {
    network.value = new NetworkIndex(bundle)
    expect(lineProps(at({line: '24b', direction: ''}))).toEqual({routeId: '7', direction: '0'})
    expect(lineProps(at({line: '24b', direction: 'intors'}))).toEqual({routeId: '7', direction: '1'})
    expect(lineProps(at({line: '24b', direction: 'auto'}))).toEqual({routeId: '7', direction: 'auto'})
  })
})
