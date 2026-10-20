import {describe, expect, it} from 'vitest'
import {modeMask, modesIn, type StopGroup, StopClusters, type StopPoint, stationRank} from '@/utils/stopClusters.ts'

const icon = {size: [24, 24] as [number, number], anchor: [12, 12] as [number, number]}
const pill = {size: [100, 40] as [number, number], anchor: [50, 15] as [number, number]}
const box = (group: StopGroup) => group.stations.size === 1 ? icon : pill

// 0.0001° of longitude is ~37 px at zoom 18 and ~5 px at zoom 15.
const point = (id: string, lng: number, station = Number(id), rank = 1): StopPoint =>
  ({id, lat: 44.43, lng: 26.1 + lng, station, modes: modeMask([3]), rank, favorite: false})

describe('stop clusters', () => {
  it('keeps stations apart while their icons do not touch', () => {
    const clusters = new StopClusters([point('1', 0), point('2', 0.0002)], box, 19, 0.25)
    expect(clusters.at(18).map((g) => g.points.length)).toEqual([1, 1])
    expect(clusters.at(15).map((g) => g.points.length)).toEqual([2])
  })

  it('sits on the highest ranked station', () => {
    const clusters = new StopClusters([point('1', 0), point('2', 0.0001, 2, 50_000)], box, 19, 0.25)
    const [group] = clusters.at(14)
    expect(clusters.points[group!.leader]!.id).toBe('2')
    const {lat, lng} = clusters.points[1]!
    expect(clusters.lngLat(group!)).toEqual({lat, lng})
  })

  it('counts platforms of one station once', () => {
    const clusters = new StopClusters([point('1', 0, 7), point('2', 0.00005, 7)], box, 19, 0.25)
    expect(clusters.at(17)[0]!.stations.size).toBe(1)
  })

  it('absorbs what its own pill and label cover', () => {
    const clusters = new StopClusters([point('1', 0), point('2', 0.00005), point('3', 0.0004)], box, 19, 0.25)
    expect(clusters.at(17).map((g) => g.points.length).sort()).toEqual([1, 2])
    expect(clusters.at(16).map((g) => g.points.length)).toEqual([3])
  })

  it('expands to where the group breaks up', () => {
    const clusters = new StopClusters([point('1', 0), point('2', 0.0002)], box, 19, 0.25)
    const [group] = clusters.at(14)
    const {split, apart} = clusters.expansion(group!, 14)
    expect(split).toBeGreaterThan(14)
    expect(apart).toBeGreaterThanOrEqual(split)
    expect(clusters.at(apart).every((g) => g.stations.size === 1)).toBe(true)
  })

  it('ranks bigger modes first, then more lines', () => {
    expect(stationRank([1])).toBeGreaterThan(stationRank([3, 3, 3, 3]))
    expect(stationRank([3, 3])).toBeGreaterThan(stationRank([3]))
    expect(modesIn(modeMask([3, 1, 0]))).toEqual(['metro', 'tram', 'bus'])
  })
})
