// This device's own data as sync records, and records written back into it.
import {type FavoritePlan, type Recent, savedFavorites, useFavoritesStore} from '@/stores/favorites'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'
import {i18n, translate} from '@/i18n'
import {chooseCity, cities, cityChoice, currentCity, knownCity} from '@/utils/city.ts'
import {
  blankEnd,
  CITY_CAP,
  type CityRecord,
  forbidden,
  LAT,
  LON,
  newestSeen,
  type NewsRecord,
  type OptionsRecord,
  type Place,
  placeId,
  type RecentItem,
  recentId,
  type RecentsRecord,
  roundCoordinate,
  type SettingsRecord,
  validId,
} from '@/utils/sync/records.ts'

// Drops what the text rule refuses, trims the ends, cuts to max code points.
export function cleanText(value: unknown, max: number): string {
  if (typeof value !== 'string') return ''
  let points = [...value.normalize('NFC')].filter((ch) => !forbidden(ch.codePointAt(0)!))
  const blank = (ch: string | undefined) => ch !== undefined && blankEnd(ch.codePointAt(0)!)
  while (blank(points[0])) points.shift()
  points = points.slice(0, max)
  while (blank(points.at(-1))) points.pop()
  return points.join('')
}

const inside = (x: number, [lo, hi]: readonly [number, number]) => x >= lo && x <= hi
const coordsText = (lat: number, lon: number) => `${lat.toFixed(4)}, ${lon.toFixed(4)}`

function toPlace(p: FavoritePlan): Place | null {
  const lat = roundCoordinate(p.lat),
    lon = roundCoordinate(p.lon)
  if (!inside(lat, LAT) || !inside(lon, LON)) return null
  const place: Place = {name: cleanText(p.name, 100) || coordsText(lat, lon), label: cleanText(p.label, 40), lat, lon}
  if (Number.isFinite(p.originLat) && Number.isFinite(p.originLon)) {
    const oLat = roundCoordinate(p.originLat!),
      oLon = roundCoordinate(p.originLon!)
    if (!inside(oLat, LAT) || !inside(oLon, LON)) return null
    place.origin = {name: cleanText(p.originName, 100) || coordsText(oLat, oLon), lat: oLat, lon: oLon}
  }
  return place
}

function toPlan(p: Place): FavoritePlan {
  const plan: FavoritePlan = {name: p.name, lat: p.lat, lon: p.lon}
  if (p.label) plan.label = p.label
  if (p.origin) Object.assign(plan, {originName: p.origin.name, originLat: p.origin.lat, originLon: p.origin.lon})
  return plan
}

function distinct<T>(items: T[], id: (item: T) => string): T[] {
  const seen = new Set<string>()
  return items.filter((item) => !seen.has(id(item)) && !!seen.add(id(item)))
}

export function cityFromLocal(slug: string): CityRecord {
  const saved = savedFavorites(slug)
  return {
    stops: distinct(saved.stops.filter(validId), String),
    lines: distinct(
      saved.routes.filter((r) => validId(r.routeId)).map((r) => ({route: r.routeId, dir: r.direction})),
      (l) => `${l.route}/${l.dir}`,
    ),
    places: distinct(
      saved.plans.map(toPlace).filter((p): p is Place => p !== null),
      placeId,
    ),
    followed: distinct(
      useRouteUpdatesStore()
        .followedIn(slug)
        .map((name) => cleanText(name, 16))
        .filter(Boolean),
      String,
    ),
  }
}

export const isEmptyCity = (r: CityRecord) => !r.stops.length && !r.lines.length && !r.places.length && !r.followed.length

export function applyCity(slug: string, r: CityRecord) {
  useFavoritesStore().importAll(
    {
      routes: r.lines.map((l) => ({routeId: l.route, direction: l.dir})),
      stops: r.stops,
      plans: r.places.map(toPlan),
      recentPlans: savedFavorites(slug).recentPlans,
    },
    slug,
  )
  useRouteUpdatesStore().importFollowed(r.followed, slug)
}

export function newsFromLocal(slug: string): NewsRecord {
  return {seen: newestSeen(distinct(useRouteUpdatesStore().seenIn(slug).filter(validId), String))}
}

export const isEmptyNews = (r: NewsRecord) => !r.seen.length

export const applyNews = (slug: string, r: NewsRecord) => useRouteUpdatesStore().importSeen(r.seen, slug)

function toRecentItem(r: Recent): RecentItem | null {
  if ('stopId' in r) return validId(r.stopId) ? {kind: 'stop', id: r.stopId} : null
  if ('routeId' in r) return validId(r.routeId) ? {kind: 'line', id: r.routeId} : null
  const place = toPlace(r)
  return place && {kind: 'place', place}
}

function toRecent(item: RecentItem): Recent {
  if (item.kind === 'place') return toPlan(item.place)
  return item.kind === 'stop' ? {stopId: item.id} : {routeId: item.id}
}

export const recentsFromLocal = (slug: string): RecentsRecord => ({
  recent: distinct(
    savedFavorites(slug)
      .recentPlans.map(toRecentItem)
      .filter((item): item is RecentItem => item !== null),
    recentId,
  ),
})

export const isEmptyRecents = (r: RecentsRecord) => !r.recent.length

export const applyRecents = (slug: string, r: RecentsRecord) => useFavoritesStore().importRecents(r.recent.map(toRecent), slug)

export const syncedCities = () => cities.value.map((c) => c.slug)

export type OverCap = {city: string; list: 'stops' | 'lines' | 'places' | 'followed'; count: number}

// Lists already past their cap, which sync will not cut on its own.
export function overCaps(): OverCap[] {
  const out: OverCap[] = []
  for (const city of syncedCities()) {
    const r = cityFromLocal(city)
    for (const list of ['stops', 'lines', 'places', 'followed'] as const) {
      if (r[list].length > CITY_CAP) out.push({city, list, count: r[list].length})
    }
  }
  return out
}

export function settingsFromLocal(): SettingsRecord {
  const s = useSettingsStore()
  return {
    theme: s.theme,
    locale: s.locale,
    autoCenterOnMe: s.autoCenterOnMe,
    autoFitMap: s.autoFitMap,
    showVehicleExtras: true,
    greenFriday: s.regionSetting('cluj', 'showGreenFriday'),
  }
}

export function applySettings(r: SettingsRecord) {
  const s = useSettingsStore()
  if (s.theme !== r.theme) s.setTheme(r.theme)
  if (s.locale !== r.locale) {
    s.setLocale(r.locale)
    i18n.global.locale.value = r.locale
  }
  if (s.autoCenterOnMe !== r.autoCenterOnMe) s.setAutoCenterOnMe(r.autoCenterOnMe)
  if (s.autoFitMap !== r.autoFitMap) s.setAutoFitMap(r.autoFitMap)
  if (s.regionSetting('cluj', 'showGreenFriday') !== r.greenFriday) s.setRegionSetting('cluj', 'showGreenFriday', r.greenFriday)
}

// What only the phone has stays as last agreed.
export function optionsFromLocal(base: OptionsRecord | null): OptionsRecord {
  return {
    colorfulLines: useSettingsStore().colorfulLines,
    city: cityChoice.value ?? '',
    journeyVoice: base?.journeyVoice ?? null,
    fullScreenAlarms: base?.fullScreenAlarms ?? null,
  }
}

async function switchCity(slug: string | null) {
  const target = await chooseCity(slug)
  if (target === currentCity.value) return
  const {default: router} = await import('@/router')
  void router.push(`/${target}/`)
}

export function applyOptions(r: OptionsRecord) {
  const s = useSettingsStore()
  if (s.colorfulLines !== r.colorfulLines) s.setColorfulLines(r.colorfulLines)
  if ((cityChoice.value ?? '') !== r.city && knownCity(r.city || currentCity.value)) void switchCity(r.city || null)
}

export function deviceLabel(): string {
  const ua = navigator.userAgent
  const browser =
    'brave' in navigator
      ? 'Brave'
      : /Edg\//.test(ua)
        ? 'Edge'
        : /OPR\//.test(ua)
          ? 'Opera'
          : /SamsungBrowser/.test(ua)
            ? 'Samsung Internet'
            : /Firefox\//.test(ua)
              ? 'Firefox'
              : /Chrome\//.test(ua)
                ? 'Chrome'
                : /Safari\//.test(ua)
                  ? 'Safari'
                  : ''
  const os = /Android/.test(ua)
    ? 'Android'
    : /iPhone|iPad|iPod/.test(ua)
      ? 'iOS'
      : /Windows/.test(ua)
        ? 'Windows'
        : /Macintosh|Mac OS X/.test(ua)
          ? 'macOS'
          : /CrOS/.test(ua)
            ? 'ChromeOS'
            : /Linux/.test(ua)
              ? 'Linux'
              : ''
  const label = browser && os ? translate('syncDeviceLabel', {browser, os}) : browser || os
  return cleanText(label, 40) || translate('syncWebDevice')
}
