import {describe, expect, it} from 'vitest'
import {getVehicleMarkerHtml, makeClusterIcon, makeSpiderIcon, makeStopIcon, type DisplayVehicle, type IconThemeOptions} from '@/utils/mapIcons.ts'
import {spiderLegs} from '@/utils/spider.ts'

const t = (key: string) => key
const bus: DisplayVehicle = {id: 1, trip_id: '25_0', route_id: 25, latitude: 46.77, longitude: 23.6, heading: 90, timestamp: '', route_short_name: '25'}
const themes: IconThemeOptions[] = [
  {arcadeActive: false, legacyBlueActive: false, paperActive: false, showVehicleExtras: true},
  {arcadeActive: true, legacyBlueActive: false, paperActive: false, showVehicleExtras: true},
  {arcadeActive: false, legacyBlueActive: true, paperActive: false, showVehicleExtras: true},
  {arcadeActive: false, legacyBlueActive: false, paperActive: true, showVehicleExtras: true},
]
const struck = '#ef4444'

describe('vehicle accessibility', () => {
  it('says nothing when the feed does not tell', () => {
    for (const theme of themes) {
      const html = getVehicleMarkerHtml(bus, '#123456', true, theme, t)
      expect(html).not.toContain(struck)
      expect(html).not.toContain('paperNo')
      expect(html).not.toContain('paperWheelchair')
    }
  })

  it('marks what the feed says', () => {
    const told = {...bus, wheelchair_accessible: 'WHEELCHAIR_ACCESSIBLE', bike_accessible: 'BIKE_INACCESSIBLE'}
    for (const theme of themes.filter(th => !th.paperActive)) {
      const html = getVehicleMarkerHtml(told, '#123456', true, theme, t)
      expect(html.split(struck).length - 1).toBe(1)
    }
    const paper = getVehicleMarkerHtml(told, '#123456', true, themes[3]!, t)
    expect(paper).toContain('paperWheelchair')
    expect(paper).toContain('paperNoBikes')
  })

  it('leaves out the half the feed is silent on', () => {
    const half = {...bus, bike_accessible: 'BIKE_ACCESSIBLE'}
    const paper = getVehicleMarkerHtml(half, '#123456', true, themes[3]!, t)
    expect(paper).toContain('paperBikes')
    expect(paper).not.toContain('Wheelchair')
  })
})

describe('station group marker', () => {
  const tram = {mode: 'tram' as const, color: '#BE1622'}

  it('wraps three icons a row in every theme', () => {
    for (const theme of themes) {
      const one = makeClusterIcon(['heart', tram], 'Piața Unirii', 1, 0, theme)
      const four = makeClusterIcon(['heart', tram, tram, tram], 'Piața Unirii', 3, 0, theme)
      expect(four.size[0]).toBe(makeClusterIcon(['heart', tram, tram], 'Piața Unirii', 2, 0, theme).size[0])
      expect(four.size[1]).toBeGreaterThan(one.size[1])
      expect(one.html).toContain('Piața Unirii')
      expect(one.html).toContain('+1')
    }
  })

  it('is as wide as its label', () => {
    const icon = makeClusterIcon([tram, tram], 'Piața Victoriei 1 (M1)', 1, 180, themes[0]!)
    expect(icon.size[0]).toBe(180)
    expect(icon.anchor[0]).toBe(90)
  })
})

describe('spider icon', () => {
  it('lays every station out with its name and a leg, in every theme', () => {
    const legs = spiderLegs(3)
    for (const theme of themes) {
      const stations = ['Guhring S', 'Guhring N', 'A & B'].map((name, i) => ({id: String(i + 1), name, icon: makeStopIcon(theme), leg: legs[i]!}))
      const icon = makeSpiderIcon(stations, theme)
      const html = icon.html as string
      expect(html.match(/data-stop="/g)).toHaveLength(3)
      expect(html.match(/<line /g)).toHaveLength(3)
      expect(html).toContain('A &amp; B')
      expect(html).toContain('flex-direction:row-reverse')
      expect(icon.anchor).toEqual([icon.size[0] / 2, icon.size[1] / 2])
    }
  })
})
