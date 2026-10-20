import {describe, expect, it} from 'vitest'
import {groupName, modeMask, modesIn, type StopGroup, StopClusters, type StopPoint, stationRank} from '@/utils/stopClusters.ts'

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
    const {split, apart} = clusters.expansion(group!, 14)!
    expect(split).toBeGreaterThan(14)
    expect(apart).toBeGreaterThanOrEqual(split)
    expect(clusters.at(apart).every((g) => g.stations.size === 1)).toBe(true)
  })

  it('has no expansion for stations that share a spot', () => {
    const clusters = new StopClusters([point('1', 0), point('2', 0.000005, 2)], box, 19, 0.25)
    const [group] = clusters.at(14)
    expect(group!.stations.size).toBe(2)
    expect(clusters.expansion(group!, 14)).toBeNull()
    const [top] = clusters.at(19)
    expect(clusters.expansion(top!, 19)).toBeNull()
  })

  it('gives every stop to its nearest marker, and no markers overlap', () => {
    let seed = 7
    const random = () => (seed = (seed * 16807) % 2147483647) / 2147483647
    const points = Array.from({length: 300}, (_, i) => ({
      ...point(String(i), random() * 0.05),
      lat: 44.43 + random() * 0.03,
      rank: Math.floor(random() * 5) * 10_000,
    }))
    const clusters = new StopClusters(points, box, 19, 0.25)
    const mx = (lng: number) => lng / 360
    const my = (lat: number) => -Math.log(Math.tan(Math.PI / 4 + lat * Math.PI / 360)) / (2 * Math.PI)
    for (const zoom of [12, 13.5, 15, 16.75, 18]) {
      const groups = clusters.at(zoom)
      const at = (i: number) => [mx(points[i]!.lng), my(points[i]!.lat)] as const
      for (const group of groups) {
        for (const i of group.points) {
          const [x, y] = at(i)
          const d = (l: number) => (at(l)[0] - x) ** 2 + (at(l)[1] - y) ** 2
          const nearest = Math.min(...groups.map((g) => d(g.leader)))
          expect(d(group.leader)).toBeCloseTo(nearest, 12)
        }
      }
      const px = 512 * 2 ** (19 - Math.ceil((19 - zoom) / 0.25 - 1e-9) * 0.25)
      const rects = groups.map((g) => {
        const {size, anchor} = box(g)
        const [x, y] = at(g.leader)
        return {left: x * px - anchor[0], top: y * px - anchor[1], right: x * px - anchor[0] + size[0], bottom: y * px - anchor[1] + size[1]}
      })
      rects.forEach((a, i) => rects.slice(i + 1).forEach((b) =>
        expect(a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom).toBe(false)))
    }
  })

  it('builds the same levels a step at a time', () => {
    let seed = 3
    const random = () => (seed = (seed * 16807) % 2147483647) / 2147483647
    const points = Array.from({length: 300}, (_, i) => ({...point(String(i), random() * 0.05), lat: 44.43 + random() * 0.03}))
    const stepped = new StopClusters(points, box, 19, 0.25)
    while (!stepped.ready(12)) stepped.advance()
    expect(stepped.ready(11.75)).toBe(false)
    const direct = new StopClusters(points, box, 19, 0.25)
    for (const zoom of [18, 15, 12]) expect(stepped.at(zoom).map((g) => g.key).sort()).toEqual(direct.at(zoom).map((g) => g.key).sort())
  })

  it('finds the markers whose boxes reach into the view', () => {
    const clusters = new StopClusters([point('1', 0), point('2', 0.01)], box, 19, 0.25)
    const ids = (west: number, east: number) => clusters.within(18, 26.1 + west, 44.4299, 26.1 + east, 44.4301).map((g) => clusters.points[g.leader]!.id)
    expect(ids(-0.0001, 0.0001)).toEqual(['1'])
    expect(ids(0.00003, 0.0002)).toEqual(['1'])
    expect(ids(0.00004, 0.0002)).toEqual([])
  })

  it('ranks bigger modes first, then more lines', () => {
    expect(stationRank([1])).toBeGreaterThan(stationRank([3, 3, 3, 3]))
    expect(stationRank([3, 3])).toBeGreaterThan(stationRank([3]))
    expect(modesIn(modeMask([3, 1, 0]))).toEqual(['metro', 'tram', 'bus'])
  })
})

describe('group names', () => {
  it('uses the root the stations share', () => {
    expect(groupName('C-tin Brâncoveanu Est', ['C-tin Brâncoveanu Vest'])).toEqual({name: 'C-tin Brâncoveanu', covered: 1})
    expect(groupName('Piața Unirii 1 (M1/3)', ['Piata Unirii 2', 'Piața Unirii', 'Piața Romană'])).toEqual({name: 'Piața Unirii', covered: 2})
    expect(groupName('Mărăști Nord', ['Mărăști Sud'])).toEqual({name: 'Mărăști', covered: 1})
    expect(groupName('Eroilor 1 (M1/3)', ['Eroilor 2', 'Eroilor (Bragadiru)'])).toEqual({name: 'Eroilor', covered: 2})
  })

  it('keeps the full name when the shared part says less than the rest', () => {
    expect(groupName('Calea Moților', ['Calea Turzii'])).toEqual({name: 'Calea Moților', covered: 0})
    expect(groupName('Piața Romană', ['Piața Victoriei 1 (M1)'])).toEqual({name: 'Piața Romană', covered: 0})
    expect(groupName('Universitate', ['Colțea'])).toEqual({name: 'Universitate', covered: 0})
  })
})
