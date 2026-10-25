import type {LocationQuery, RouteLocationRaw} from 'vue-router'
import {searchPlaces} from '@/utils/places.ts'
import {translate as t} from '@/i18n.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {isSiteHost} from '@/utils/site.ts'
import {currentCity} from '@/utils/city.ts'

export const SHARE_PARAMS = ['shared_title', 'shared_text', 'shared_url'] as const

const param = (query: LocationQuery, key: string) => (typeof query[key] === 'string' ? (query[key] as string) : '')

export const isShare = (query: LocationQuery) => SHARE_PARAMS.some((key) => param(query, key))

export async function sharedDestination(query: LocationQuery): Promise<RouteLocationRaw> {
  const text = SHARE_PARAMS.map((key) => param(query, key))
    .filter(Boolean)
    .join('\n')
  const link = /https?:\/\/\S+/.exec(text)?.[0]
  if (link) {
    try {
      const url = new URL(link)
      if (isSiteHost(url.host)) return url.pathname + url.search
    } catch {}
  }
  const name =
    text
      .split('\n')
      .map((line) => line.trim())
      .find((line) => line && !line.startsWith('http')) ?? text.trim()
  if (!name) return {name: 'home', params: {city: currentCity.value}}
  const settings = useSettingsStore()
  settings.showToast(t('shareFinding', {name: name.slice(0, 40)}))
  const [place] = await searchPlaces(name, 1).catch(() => [])
  if (place) return {name: 'plan', params: {city: currentCity.value}, query: {lat: place.lat, lon: place.lon, name: place.label}}
  settings.showToast(t('shareNotFound'))
  return {name: 'plan', params: {city: currentCity.value}, query: {q: name}}
}
