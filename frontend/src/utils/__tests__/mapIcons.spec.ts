import {describe, expect, it} from 'vitest'
import {getVehicleMarkerHtml, makeClusterIcon, makeSpiderIcon, makeStopIcon, type DisplayVehicle} from '@/utils/mapIcons.ts'
import {spiderLegs} from '@/utils/spider.ts'

const bus: DisplayVehicle = {id: 1, trip_id: '25_0', route_id: 25, latitude: 46.77, longitude: 23.6, heading: 90, timestamp: '', route_short_name: '25'}
const struck = '#ef4444'
const accessibilityIcon = /<svg width="12" height="12"/g

describe('vehicle accessibility', () => {
  it('says nothing when the feed does not tell', () => {
    const html = getVehicleMarkerHtml(bus, '#123456', true, true)
    expect(html).not.toContain(struck)
    expect(html.match(accessibilityIcon)).toBeNull()
  })

  it('marks what the feed says', () => {
    const told = {...bus, wheelchair_accessible: 'WHEELCHAIR_ACCESSIBLE', bike_accessible: 'BIKE_INACCESSIBLE'}
    const html = getVehicleMarkerHtml(told, '#123456', true, true)
    expect(html.split(struck).length - 1).toBe(1)
    expect(html.match(accessibilityIcon)).toHaveLength(2)
  })

  it('leaves out the half the feed is silent on', () => {
    const half = {...bus, bike_accessible: 'BIKE_ACCESSIBLE'}
    const html = getVehicleMarkerHtml(half, '#123456', true, true)
    expect(html).not.toContain(struck)
    expect(html.match(accessibilityIcon)).toHaveLength(1)
  })

  it('shows nothing when the extras are off', () => {
    const told = {...bus, wheelchair_accessible: 'WHEELCHAIR_ACCESSIBLE', bike_accessible: 'BIKE_INACCESSIBLE'}
    expect(getVehicleMarkerHtml(told, '#123456', true, false).match(accessibilityIcon)).toBeNull()
  })
})

describe('station group marker', () => {
  const tram = {mode: 'tram' as const, color: '#BE1622'}

  it('puts up to three icons in a row, more in two even rows', () => {
    const icons = (n: number) => Array.from({length: n}, () => tram)
    const one = makeClusterIcon(['heart', tram], 'Piața Unirii', 1, 0)
    const width = (n: number) => makeClusterIcon(icons(n), '', 0, 0).size[0]
    expect(width(4)).toBe(width(2))
    expect(width(5)).toBe(width(3))
    expect(width(6)).toBe(width(3))
    expect(width(7)).toBeGreaterThan(width(3))
    expect(makeClusterIcon(icons(4), '', 0, 0).size[1]).toBeGreaterThan(one.size[1])
    expect(one.html).toContain('Piața Unirii')
    expect(one.html).toContain('+1')
  })

  it('is as wide as its label', () => {
    const icon = makeClusterIcon([tram, tram], 'Piața Victoriei 1 (M1)', 1, 180)
    expect(icon.size[0]).toBe(180)
    expect(icon.anchor[0]).toBe(90)
  })
})

describe('spider icon', () => {
  it('lays every station out with its name and a leg', () => {
    const legs = spiderLegs(3)
    const stations = ['Guhring S', 'Guhring N', 'A & B'].map((name, i) => ({id: String(i + 1), name, icon: makeStopIcon(), leg: legs[i]!}))
    const icon = makeSpiderIcon(stations)
    const html = icon.html as string
    expect(html.match(/data-stop="/g)).toHaveLength(3)
    expect(html.match(/<line /g)).toHaveLength(3)
    expect(html).toContain('A &amp; B')
    expect(html).toContain('flex-direction:row-reverse')
    expect(icon.anchor).toEqual([icon.size[0] / 2, icon.size[1] / 2])
  })
})
