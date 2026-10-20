import {describe, expect, it} from 'vitest'
import {getVehicleMarkerHtml, type DisplayVehicle, type IconThemeOptions} from '@/utils/mapIcons.ts'

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
