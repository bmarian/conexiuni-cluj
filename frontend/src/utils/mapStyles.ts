import type {StyleSpecification} from 'maplibre-gl'
import dark from '@/assets/map-styles/dark.json'
import light from '@/assets/map-styles/light.json'
import {cityInfo, cityParams} from '@/utils/city.ts'
import {escapeHtml} from '@/utils/text.ts'

const DARK = dark as unknown as StyleSpecification
const LIGHT = light as unknown as StyleSpecification

export function mapStyle(isDark: boolean): StyleSpecification {
  return isDark ? DARK : LIGHT
}

const cityCredits = () => [...cityInfo()?.credits ?? [], {name: cityParams().operator, url: cityInfo()?.operatorUrl ?? ''}].filter((credit) => credit.name)

const creditLink = (name: string, url: string) => url
  ? `<a href="${escapeHtml(url)}" target="_blank" rel="noopener">${escapeHtml(name)}</a>`
  : escapeHtml(name)

export const mapCreditsHTML = () => `&copy; ${creditLink('OpenMapTiles', 'https://openmaptiles.org/')} &copy; ${creditLink('OpenStreetMap', 'https://www.openstreetmap.org/copyright')} | `
  + cityCredits().map((credit) => '&copy; ' + creditLink(credit.name, credit.url)).join(', ')
export const mapCreditsText = () => `© OpenMapTiles © OpenStreetMap | ${cityCredits().map((credit) => '© ' + credit.name).join(', ')}`
