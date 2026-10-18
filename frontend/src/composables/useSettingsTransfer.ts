import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import {parseFavoriteRoutes, parsePlans, parseStopIds, useFavoritesStore} from '@/stores/favorites'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'
import {REGION_SETTINGS, regionSettingsFor} from '@/utils/regionSettings.ts'
import {cities} from '@/utils/city.ts'

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

// Summary shown before a received export replaces the current one.
export function describeExport(data: unknown) {
  if (!isRecord(data) || (!isRecord(data.favorites) && !isRecord(data.settings))) return null
  const favorites = isRecord(data.favorites) ? data.favorites : {}
  return {
    stops: parseStopIds(favorites.stops).length,
    lines: parseFavoriteRoutes(favorites.routes).length,
    places: parsePlans(favorites.plans).length,
    followed: Array.isArray(data.followedLines) ? data.followedLines.filter((x) => typeof x === 'string').length : 0,
  }
}

// The same export goes out as text (Export / Import) and through a transfer code.
function regionsExport() {
  const settings = useSettingsStore()
  const out: Record<string, Record<string, boolean>> = {}
  const slugs = new Set([...cities.value.map((c) => c.slug), ...REGION_SETTINGS.flatMap((s) => s.cities)])
  for (const slug of slugs) {
    for (const setting of regionSettingsFor(slug)) {
      ;(out[slug] ??= {})[setting.key] = settings.regionSetting(slug, setting.key)
    }
  }
  return out
}

export function useSettingsTransfer() {
  const {locale} = useI18n()
  const settings = useSettingsStore()
  const favs = useFavoritesStore()
  const routeUpdates = useRouteUpdatesStore()

  function buildExport() {
    return {
      version: 1,
      settings: {
        theme: settings.theme,
        locale: settings.locale,
        arcadeUnlocked: settings.arcadeUnlocked,
        arcadeActive: settings.arcadeActive,
        legacyBlueUnlocked: settings.legacyBlueUnlocked,
        legacyBlueActive: settings.legacyBlueActive,
        paperActive: settings.paperActive,
        autoCenterOnMe: settings.autoCenterOnMe,
        autoFitMap: settings.autoFitMap,
        regions: regionsExport(),
        busAlerts: settings.busAlerts,
      },
      favorites: {
        routes: favs.favoriteRoutes,
        stops: favs.favoriteStopIds,
        plans: favs.favoritePlans,
        recentPlans: favs.recentPlans,
      },
      followedLines: routeUpdates.followed,
    }
  }

  function applyLocale(value: unknown) {
    if (value !== 'ro' && value !== 'en') return
    settings.setLocale(value)
    locale.value = value
  }

  // Throws when the data isn't an export.
  function applyExport(data: unknown) {
    if (!isRecord(data)) throw new Error('not a settings export')
    const s = (isRecord(data.settings) ? data.settings : {}) as Partial<ReturnType<typeof buildExport>['settings']>
    if (s.theme === 'light' || s.theme === 'dark' || s.theme === 'system') settings.setTheme(s.theme)
    applyLocale(s.locale)
    if (s.arcadeUnlocked) settings.unlockArcade()
    if (s.arcadeActive) settings.activateArcade()
    else settings.deactivateArcade()
    if (s.legacyBlueUnlocked) settings.unlockLegacyBlue()
    if (s.legacyBlueActive) settings.activateLegacyBlue()
    else settings.deactivateLegacyBlue()
    if (s.paperActive) settings.activatePaper()
    else settings.deactivatePaper()
    if (typeof s.autoCenterOnMe === 'boolean') settings.setAutoCenterOnMe(s.autoCenterOnMe)
    if (typeof s.autoFitMap === 'boolean') settings.setAutoFitMap(s.autoFitMap)
    if (s.regions && typeof s.regions === 'object') {
      for (const [slug, values] of Object.entries(s.regions as Record<string, Record<string, unknown>>)) {
        for (const setting of regionSettingsFor(slug)) {
          if (typeof values?.[setting.key] === 'boolean') settings.setRegionSetting(slug, setting.key, values[setting.key] as boolean)
        }
      }
    }
    if (typeof s.busAlerts === 'boolean') settings.setBusAlerts(s.busAlerts)
    favs.importAll(isRecord(data.favorites) ? data.favorites : {})
    if (Array.isArray(data.followedLines)) routeUpdates.importFollowed(data.followedLines)
  }

  // The simplified layout keeps its own look, so it only takes favorites and the language.
  function applyFavorites(data: unknown) {
    if (!isRecord(data)) throw new Error('not a settings export')
    favs.importAll(isRecord(data.favorites) ? data.favorites : {})
    applyLocale(isRecord(data.settings) ? data.settings.locale : undefined)
  }

  return {buildExport, applyExport, applyFavorites}
}
