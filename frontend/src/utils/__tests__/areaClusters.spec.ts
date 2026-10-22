import {describe, expect, it} from 'vitest'
import {type Area, AreaClusters} from '@/utils/areaClusters.ts'
import {modeMask, StopClusters, type StopPoint} from '@/utils/stopClusters.ts'

const box = () => ({size: [100, 40] as [number, number], anchor: [50, 15] as [number, number]})

// 1 km of longitude here is ~0.0132°.
const KM = 0.0132

const stop = (id: number, lat: number, lng: number): StopPoint => ({id: `${id}`, lat, lng, station: id, modes: modeMask([3]), rank: 1, favorite: false})

const clustersOf = (areas: Area[], points: StopPoint[]) => new AreaClusters(new StopClusters(points, box, 19, 0.25), areas, box)

const names = (clusters: AreaClusters, zoom: number) => clusters.at(zoom).map((g) => clusters.name(g))

describe('area clusters', () => {
  const areas: Area[] = [
    {name: 'Cluj-Napoca', lat: 46.770, lon: 23.590, place: 'city'},
    {name: 'Mănăștur', lat: 46.755, lon: 23.557, place: 'suburb'},
    {name: 'Florești', lat: 46.744, lon: 23.485, place: 'village'},
  ]
  const points = areas.flatMap((a, i) => [0, 1, 2].map((k) => stop(i * 10 + k, a.lat + k * 0.0005, a.lon + k * 0.0005)))

  it('starts with every area on its own stops', () => {
    expect(names(clustersOf(areas, points), 19)).toEqual(['Cluj-Napoca', 'Mănăștur', 'Florești'])
  })

  it('names the whole map after the biggest town from far out', () => {
    expect(names(clustersOf(areas, points), 8)).toEqual(['Cluj-Napoca'])
  })

  it('never lets a smaller place name the stops of a bigger one', () => {
    const line: Area[] = [
      {name: 'City', lat: 46.77, lon: 23.59, place: 'city'},
      {name: 'Suburb', lat: 46.77, lon: 23.59 - KM, place: 'suburb'},
      {name: 'Neighbourhood', lat: 46.77, lon: 23.59 - 2.4 * KM, place: 'neighbourhood'},
    ]
    // The suburb's stops lie nearer the neighbourhood than the city.
    const lngs = [23.59, 23.59 - 1.6 * KM, 23.59 - 2.4 * KM]
    const clusters = clustersOf(line, lngs.flatMap((lng, i) => [0, 1, 2].map((k) => stop(i * 3 + k, 46.77, lng))))
    const zoom = [...Array(45).keys()].map((l) => 19 - l * 0.25).find((z) => !names(clusters, z).includes('Suburb'))!
    expect(names(clusters, zoom)).toContain('Neighbourhood')
    const city = clusters.at(zoom).find((g) => clusters.name(g) === 'City')!
    expect(city.points).toEqual(expect.arrayContaining([3, 4, 5]))
  })
})
