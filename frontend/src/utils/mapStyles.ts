import type {StyleSpecification} from 'maplibre-gl'
import dark from '@/assets/map-styles/dark.json'
import light from '@/assets/map-styles/light.json'
import {cityInfo, cityParams} from '@/utils/city.ts'

const DARK = dark as unknown as StyleSpecification
const LIGHT = light as unknown as StyleSpecification

type Curve = { r: number[], g: number[], b: number[] }

// Paper prints the dark map in greyscale, then maps each grey onto ink and paper per channel.
const PAPER_LIGHT: Curve = {
  r: [0.937, 0.888, 0.591, 0.394, 0.278, 0.213, 0.180, 0.155, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114, 0.114],
  g: [0.898, 0.857, 0.610, 0.445, 0.349, 0.294, 0.267, 0.246, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212, 0.212],
  b: [0.792, 0.771, 0.647, 0.564, 0.516, 0.488, 0.475, 0.464, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447, 0.447],
}
const PAPER_DARK: Curve = {
  r: [0.106, 0.156, 0.455, 0.655, 0.771, 0.837, 0.871, 0.896, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937, 0.937],
  g: [0.145, 0.190, 0.461, 0.642, 0.747, 0.808, 0.838, 0.860, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898, 0.898],
  b: [0.275, 0.306, 0.492, 0.616, 0.689, 0.730, 0.751, 0.766, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792, 0.792],
}

type Rgba = [number, number, number, number]

function hslToRgb(h: number, s: number, l: number): [number, number, number] {
  const k = (n: number) => (n + h / 30) % 12
  const a = s * Math.min(l, 1 - l)
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, 9 - k(n), 1))
  return [f(0), f(8), f(4)]
}

function parseColor(value: string): Rgba | null {
  const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(value.trim())
  if (hex) {
    const digits = hex[1]!.length === 3 ? [...hex[1]!].map((c) => c + c).join('') : hex[1]!
    return [0, 2, 4].map((i) => parseInt(digits.slice(i, i + 2), 16) / 255).concat(1) as Rgba
  }
  const fn = /^(rgb|hsl)a?\(([^)]*)\)$/i.exec(value.trim())
  if (!fn) return null
  const parts = fn[2]!.split(/[\s,/]+/).filter(Boolean).map((p) => parseFloat(p))
  if (parts.length < 3 || parts.some((p) => !Number.isFinite(p))) return null
  const alpha = parts[3] ?? 1
  if (fn[1]!.toLowerCase() === 'rgb') return [parts[0]! / 255, parts[1]! / 255, parts[2]! / 255, alpha]
  return [...hslToRgb(parts[0]!, parts[1]! / 100, parts[2]! / 100), alpha]
}

function table(values: number[], c: number): number {
  const n = values.length - 1
  const k = Math.min(Math.floor(c * n), n - 1)
  return values[k]! + (c * n - k) * (values[k + 1]! - values[k]!)
}

function inked(value: string, curve: Curve): string {
  const rgba = parseColor(value)
  if (!rgba) return value
  const [r, g, b, a] = rgba
  const grey = 0.213 * r + 0.715 * g + 0.072 * b
  const channel = (values: number[]) => Math.round(table(values, grey) * 255)
  return `rgba(${channel(curve.r)},${channel(curve.g)},${channel(curve.b)},${a})`
}

function inkValue(value: unknown, curve: Curve): unknown {
  if (typeof value === 'string') return inked(value, curve)
  if (Array.isArray(value)) return value.map((v) => inkValue(v, curve))
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, inkValue(v, curve)]))
  }
  return value
}

type StyleLayer = StyleSpecification['layers'][number]

function recolored(colorOf: (layer: StyleLayer, key: string, value: unknown) => unknown): StyleSpecification {
  return {
    ...DARK,
    layers: DARK.layers.map((layer) => {
      if (!('paint' in layer) || !layer.paint) return layer
      const paint = Object.fromEntries(Object.entries(layer.paint).map(([key, value]) =>
        [key, key.endsWith('-color') ? colorOf(layer, key, value) : value]))
      return {...layer, paint} as typeof layer
    }),
  }
}

type Role = 'land' | 'residential' | 'green' | 'water' | 'building' | 'path' | 'minor' | 'major' | 'motorway'
  | 'casing' | 'rail' | 'boundary' | 'label' | 'roadLabel' | 'waterLabel'
type Palette = Record<Role, string> & { halo: string, buildingLine: string }

const LAYER_ROLES: Record<string, Role> = {
  'background': 'land',
  'water': 'water',
  'landcover_ice_shelf': 'land',
  'landcover_glacier': 'land',
  'landuse_residential': 'residential',
  'landcover_wood': 'green',
  'landuse_park': 'green',
  'waterway': 'water',
  'water_name': 'waterLabel',
  'building': 'building',
  'aeroway-taxiway': 'minor',
  'aeroway-runway-casing': 'casing',
  'aeroway-area': 'land',
  'aeroway-runway': 'minor',
  'road_area_pier': 'land',
  'road_pier': 'land',
  'highway_path': 'path',
  'highway_minor': 'minor',
  'highway_major_casing': 'casing',
  'highway_major_inner': 'major',
  'highway_major_subtle': 'major',
  'highway_motorway_casing': 'casing',
  'highway_motorway_inner': 'motorway',
  'highway_motorway_subtle': 'motorway',
  'railway_transit': 'rail',
  'railway_minor': 'rail',
  'railway': 'rail',
  // The dashes are gaps painted in the land colour.
  'railway_transit_dashline': 'land',
  'railway_minor_dashline': 'land',
  'railway_dashline': 'land',
  'highway_name_other': 'roadLabel',
  'highway_name_motorway': 'roadLabel',
}

const roleOf = (id: string): Role | undefined =>
  LAYER_ROLES[id] ?? (id.startsWith('boundary_') ? 'boundary' : id.startsWith('place_') ? 'label' : undefined)

function paletteStyle(palette: Palette): StyleSpecification {
  return recolored((layer, key, value) => {
    const role = roleOf(layer.id)
    if (!role) return value
    if (key === 'text-halo-color') return palette.halo
    if (key === 'fill-outline-color') return palette.buildingLine
    return palette[role]
  })
}

// Colours from each theme's stylesheet; arcade's main roads are hollow corridors.
const ARCADE_LIGHT: Palette = {
  land: '#fefce8', residential: '#fef9c3', green: '#fef3c7', water: '#fde68a',
  building: '#fef3c7', buildingLine: '#fde68a',
  path: '#fef3c7', minor: '#fde68a', major: '#fefce8', motorway: '#fde68a', casing: '#f59e0b',
  rail: '#fcd34d', boundary: '#fbbf24',
  label: '#92400e', roadLabel: '#b45309', waterLabel: '#b45309', halo: '#fefce8',
}
const ARCADE_DARK: Palette = {
  land: '#1c1608', residential: '#211a05', green: '#211a05', water: '#2a2006',
  building: '#211a05', buildingLine: '#2a2006',
  path: '#211a05', minor: '#422006', major: '#1c1608', motorway: '#422006', casing: '#d97706',
  rail: '#422006', boundary: '#78350f',
  label: '#d97706', roadLabel: '#a16207', waterLabel: '#a16207', halo: '#1c1608',
}
const LEGACY_LIGHT: Palette = {
  land: '#ece9d8', residential: '#ece9d8', green: '#b8c8b0', water: '#8fb3dc',
  building: '#d4d0c8', buildingLine: '#aca899',
  path: '#d4d0c8', minor: '#ffffff', major: '#ffe07a', motorway: '#f3c94e', casing: '#aca899',
  rail: '#808080', boundary: '#aca899',
  label: '#000000', roadLabel: '#404040', waterLabel: '#003c9c', halo: '#ffffff',
}
const LEGACY_DARK: Palette = {
  land: '#1a2030', residential: '#1a2030', green: '#2d4520', water: '#0e1b40',
  building: '#232a40', buildingLine: '#2a2d38',
  path: '#232a40', minor: '#2a2d38', major: '#2a2d38', motorway: '#444a5c', casing: '#444a5c',
  rail: '#444a5c', boundary: '#444a5c',
  label: '#e0e6f2', roadLabel: '#8898b0', waterLabel: '#6aa0e8', halo: '#1a2030',
}

export type MapTheme = 'default' | 'paper' | 'arcade' | 'legacyBlue'

const cache = new Map<string, StyleSpecification>()

const build: Record<Exclude<MapTheme, 'default'>, (isDark: boolean) => StyleSpecification> = {
  paper: (isDark) => recolored((_layer, _key, value) => inkValue(value, isDark ? PAPER_DARK : PAPER_LIGHT)),
  arcade: (isDark) => paletteStyle(isDark ? ARCADE_DARK : ARCADE_LIGHT),
  legacyBlue: (isDark) => paletteStyle(isDark ? LEGACY_DARK : LEGACY_LIGHT),
}

export function mapStyle(isDark: boolean, theme: MapTheme): StyleSpecification {
  if (theme === 'default') return isDark ? DARK : LIGHT
  const key = `${theme}-${isDark}`
  if (!cache.has(key)) cache.set(key, build[theme](isDark))
  return cache.get(key)!
}

// Live positions come through tranzy.ai; a schedule-only city credits its operator alone.
export const mapCreditsHTML = () => '&copy; <a href="https://openmaptiles.org/" target="_blank" rel="noopener">OpenMapTiles</a> &copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> | '
  + (cityInfo()?.realtime ? '&copy; <a href="https://tranzy.ai/" target="_blank" rel="noopener">tranzy.ai</a>, ' : '') + '&copy; ' + escapeHTML(cityParams().operator)
export const mapCreditsText = () => `© OpenMapTiles © OpenStreetMap | ${cityInfo()?.realtime ? '© tranzy.ai, ' : ''}© ${cityParams().operator}`

const escapeHTML = (text: string) => text.replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c] ?? c))
