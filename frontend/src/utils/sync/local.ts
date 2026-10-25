// This device's own data as sync records, and records written back into it.
import {type FavoritePlan, savedFavorites, useFavoritesStore} from '@/stores/favorites'
import {useRouteUpdatesStore} from '@/stores/routeUpdates'
import {useSettingsStore} from '@/stores/settings'
import {i18n, translate} from '@/i18n'
import {cities} from '@/utils/city.ts'
import {
  blankEnd,
  CITY_CAP,
  type CityRecord,
  forbidden,
  LAT,
  LON,
  NEWS_CAP,
  type NewsRecord,
  type Place,
  placeId,
  roundCoordinate,
  type SettingsRecord,
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
  const lat = roundCoordinate(p.lat), lon = roundCoordinate(p.lon)
  if (!inside(lat, LAT) || !inside(lon, LON)) return null
  const place: Place = {name: cleanText(p.name, 100) || coordsText(lat, lon), label: cleanText(p.label, 40), lat, lon}
  if (Number.isFinite(p.originLat) && Number.isFinite(p.originLon)) {
    const oLat = roundCoordinate(p.originLat!), oLon = roundCoordinate(p.originLon!)
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
    stops: distinct(saved.stops.filter((id) => Number.isInteger(id) && id >= 1 && id <= 2 ** 31 - 1), String),
    lines: distinct(saved.routes.filter((r) => Number.isInteger(r.routeId) && r.routeId >= 1 && r.routeId <= 2 ** 31 - 1)
      .map((r) => ({route: r.routeId, dir: r.direction})), (l) => `${l.route}/${l.dir}`),
    places: distinct(saved.plans.map(toPlace).filter((p): p is Place => p !== null), placeId),
    followed: distinct(useRouteUpdatesStore().followedIn(slug).map((name) => cleanText(name, 16)).filter(Boolean), String),
  }
}

export const isEmptyCity = (r: CityRecord) => !r.stops.length && !r.lines.length && !r.places.length && !r.followed.length

export function applyCity(slug: string, r: CityRecord) {
  useFavoritesStore().importAll({
    routes: r.lines.map((l) => ({routeId: l.route, direction: l.dir})),
    stops: r.stops,
    plans: r.places.map(toPlan),
    recentPlans: savedFavorites(slug).recentPlans,
  }, slug)
  useRouteUpdatesStore().importFollowed(r.followed, slug)
}

export function newsFromLocal(slug: string): NewsRecord {
  const seen = useRouteUpdatesStore().seenIn(slug).filter((id) => Number.isInteger(id) && id >= 1 && id <= 2 ** 31 - 1)
  return {seen: distinct(seen, String).sort((a, b) => b - a).slice(0, NEWS_CAP)}
}

export const isEmptyNews = (r: NewsRecord) => !r.seen.length

export const applyNews = (slug: string, r: NewsRecord) => useRouteUpdatesStore().importSeen(r.seen, slug)

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

export function deviceLabel(): string {
  const ua = navigator.userAgent
  const browser = 'brave' in navigator ? 'Brave'
    : /Edg\//.test(ua) ? 'Edge'
      : /OPR\//.test(ua) ? 'Opera'
        : /SamsungBrowser/.test(ua) ? 'Samsung Internet'
          : /Firefox\//.test(ua) ? 'Firefox'
            : /Chrome\//.test(ua) ? 'Chrome'
              : /Safari\//.test(ua) ? 'Safari' : ''
  const os = /Android/.test(ua) ? 'Android'
    : /iPhone|iPad|iPod/.test(ua) ? 'iOS'
      : /Windows/.test(ua) ? 'Windows'
        : /Macintosh|Mac OS X/.test(ua) ? 'macOS'
          : /CrOS/.test(ua) ? 'ChromeOS'
            : /Linux/.test(ua) ? 'Linux' : ''
  const label = browser && os ? translate('syncDeviceLabel', {browser, os}) : browser || os
  return cleanText(label, 40) || translate('syncWebDevice')
}
