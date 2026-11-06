import {useSettingsStore} from '@/stores/settings.ts'
import {useMapStore} from '@/stores/map.ts'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {network} from '@/utils/network.ts'

export function ownColor(name: string, color: string, apart = false): string {
  const own = apart || useSettingsStore().colorfulLines || useFavoritesStore().favoriteLineNames.has(name)
  return (own && network.value?.routeByShortName.get(name)?.route_colorful) || color
}

// Badges of lines drawn on the map take the map's colour, as its legend.
export function displayColor(name: string, color: string, apart = false): string {
  return useMapStore().lineColors.get(name) ?? ownColor(name, color, apart)
}

export const manyLines = (names: string[]) => new Set(names.map((n) => n.trim()).filter(Boolean)).size > 1

const WCAG_AA = 4.5

function whiteReadable(hex: string): boolean {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return true
  const n = parseInt(m[1]!, 16)
  const linear = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  const y = 0.2126 * linear(((n >> 16) & 255) / 255) + 0.7152 * linear(((n >> 8) & 255) / 255) + 0.0722 * linear((n & 255) / 255)
  return 1.05 / (y + 0.05) >= WCAG_AA
}

export const lineInk = (color: string) => (whiteReadable(color) ? color : 'var(--md-on-surface)')
