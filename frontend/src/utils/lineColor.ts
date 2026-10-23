import {useSettingsStore} from '@/stores/settings.ts'
import {useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'

const utf8 = new TextEncoder()

export function hashedLineColor(name: string): string {
  let hash = 0x811c9dc5
  for (const byte of utf8.encode(name.trim().toUpperCase())) hash = Math.imul(hash ^ byte, 0x01000193) >>> 0
  return hslToHex(hash % 360, 66 + ((hash >>> 9) % 16), 44 + ((hash >>> 17) % 12))
}

export function ownColor(name: string, color: string, apart = false): string {
  const own = apart || useSettingsStore().colorfulLines || useFavoritesStore().favoriteLineNames.has(name)
  return own && name.trim() ? hashedLineColor(name) : color
}

// Badges of lines drawn on the map take the map's colour, as its legend.
export function displayColor(name: string, color: string, apart = false): string {
  return useMapStore().lineColors.get(name) ?? ownColor(name, color, apart)
}

export const manyLines = (names: string[]) => new Set(names.map((n) => n.trim()).filter(Boolean)).size > 1

function hslToHex(h: number, s: number, l: number): string {
  const sf = s / 100
  const lf = l / 100
  const q = lf < 0.5 ? lf * (1 + sf) : lf + sf - lf * sf
  const p = 2 * lf - q
  const hue = h / 360
  return '#' + [hue + 1 / 3, hue, hue - 1 / 3]
    .map((t) => Math.round(hueToRgb(p, q, t) * 255).toString(16).toUpperCase().padStart(2, '0'))
    .join('')
}

function hueToRgb(p: number, q: number, t: number): number {
  if (t < 0) t += 1
  if (t > 1) t -= 1
  if (t < 1 / 6) return p + (q - p) * 6 * t
  if (t < 1 / 2) return q
  if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6
  return p
}
