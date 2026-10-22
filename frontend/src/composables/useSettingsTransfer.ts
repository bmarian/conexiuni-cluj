import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import {FAVORITE_KEYS, parseFavoriteRoutes, parsePlans, parseStopIds, savedFavorites, useFavoritesStore} from '@/stores/favorites'
import {FOLLOWS_KEY, useRouteUpdatesStore} from '@/stores/routeUpdates'
import {REGION_SETTINGS, regionSettingsFor} from '@/utils/regionSettings.ts'
import {cities, citiesSaving, DEFAULT_CITY, knownCity} from '@/utils/city.ts'

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

const citiesWithFavorites = () => new Set([...FAVORITE_KEYS, FOLLOWS_KEY].flatMap(citiesSaving))

// Exports without cities, from the apps or older builds, are Cluj's.
function exportedCities(data: Record<string, unknown>): Record<string, Record<string, unknown>> {
  if (!isRecord(data.cities)) return {[DEFAULT_CITY]: {favorites: data.favorites, followedLines: data.followedLines}}
  const out: Record<string, Record<string, unknown>> = {}
  for (const [slug, entry] of Object.entries(data.cities)) {
    if (/^[a-z][a-z0-9-]*$/.test(slug) && knownCity(slug) && isRecord(entry)) out[slug] = entry
  }
  return out
}

// Summary shown before a received export replaces the current one.
export function describeExport(data: unknown) {
  if (!isRecord(data) || (!isRecord(data.favorites) && !isRecord(data.settings))) return null
  const summary = {stops: 0, lines: 0, places: 0, followed: 0}
  for (const entry of Object.values(exportedCities(data))) {
    const favorites = isRecord(entry.favorites) ? entry.favorites : {}
    summary.stops += parseStopIds(favorites.stops).length
    summary.lines += parseFavoriteRoutes(favorites.routes).length
    summary.places += parsePlans(favorites.plans).length
    summary.followed += Array.isArray(entry.followedLines) ? entry.followedLines.filter((x) => typeof x === 'string').length : 0
  }
  return summary
}

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

// The same export goes out as text (Export / Import) and through a transfer code.
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
        autoCenterOnMe: settings.autoCenterOnMe,
        autoFitMap: settings.autoFitMap,
        regions: regionsExport(),
      },
      // The apps read these and know only Cluj.
      favorites: savedFavorites(DEFAULT_CITY),
      followedLines: routeUpdates.followedIn(DEFAULT_CITY),
      cities: Object.fromEntries([...citiesWithFavorites()].map((city) => [city, {
        favorites: savedFavorites(city),
        followedLines: routeUpdates.followedIn(city),
      }])),
    }
  }

  function importFavorites(data: Record<string, unknown>, withFollowed: boolean) {
    const byCity = exportedCities(data)
    const targets = isRecord(data.cities) ? new Set([...Object.keys(byCity), ...citiesWithFavorites()]) : new Set(Object.keys(byCity))
    for (const city of targets) {
      const entry = byCity[city] ?? {}
      favs.importAll(isRecord(entry.favorites) ? entry.favorites : {}, city)
      if (!withFollowed || (city in byCity && !Array.isArray(entry.followedLines))) continue
      routeUpdates.importFollowed(entry.followedLines, city)
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
    if (typeof s.autoCenterOnMe === 'boolean') settings.setAutoCenterOnMe(s.autoCenterOnMe)
    if (typeof s.autoFitMap === 'boolean') settings.setAutoFitMap(s.autoFitMap)
    // Exports from before cities hold Cluj's settings at the top level.
    const regions = isRecord(s.regions) ? s.regions : {[DEFAULT_CITY]: s}
    for (const [slug, values] of Object.entries(regions)) {
      if (!isRecord(values)) continue
      for (const setting of regionSettingsFor(slug)) {
        if (typeof values[setting.key] === 'boolean') settings.setRegionSetting(slug, setting.key, values[setting.key] as boolean)
      }
    }
    importFavorites(data, true)
  }

  return {buildExport, applyExport}
}
