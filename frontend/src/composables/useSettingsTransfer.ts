import {useI18n} from 'vue-i18n'
import {useSettingsStore} from '@/stores/settings'
import {useFavoritesStore} from '@/stores/favorites'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

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
        arcadeUnlocked: settings.arcadeUnlocked,
        arcadeActive: settings.arcadeActive,
        legacyBlueUnlocked: settings.legacyBlueUnlocked,
        legacyBlueActive: settings.legacyBlueActive,
        paperActive: settings.paperActive,
        showWeather: settings.showWeather,
        showNews: settings.showNews,
        autoCenterOnMe: settings.autoCenterOnMe,
        autoFitMap: settings.autoFitMap,
        showGreenFriday: settings.showGreenFriday,
        showTimetableChanges: settings.showTimetableChanges,
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
    if (s.theme) settings.setTheme(s.theme)
    applyLocale(s.locale)
    if (s.arcadeUnlocked) settings.unlockArcade()
    if (s.arcadeActive) settings.activateArcade()
    else settings.deactivateArcade()
    if (s.legacyBlueUnlocked) settings.unlockLegacyBlue()
    if (s.legacyBlueActive) settings.activateLegacyBlue()
    else settings.deactivateLegacyBlue()
    if (s.paperActive) settings.activatePaper()
    else settings.deactivatePaper()
    if (typeof s.showWeather === 'boolean') settings.setShowWeather(s.showWeather)
    if (typeof s.showNews === 'boolean') settings.setShowNews(s.showNews)
    if (typeof s.autoCenterOnMe === 'boolean') settings.setAutoCenterOnMe(s.autoCenterOnMe)
    if (typeof s.autoFitMap === 'boolean') settings.setAutoFitMap(s.autoFitMap)
    if (typeof s.showGreenFriday === 'boolean') settings.setShowGreenFriday(s.showGreenFriday)
    if (typeof s.showTimetableChanges === 'boolean') settings.setShowTimetableChanges(s.showTimetableChanges)
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
