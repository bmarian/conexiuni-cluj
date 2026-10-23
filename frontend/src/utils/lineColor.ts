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
