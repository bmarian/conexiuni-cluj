import {computed} from 'vue'
import {storeToRefs} from 'pinia'
import {type FavoritePlan, type Recent, recentKey, useFavoritesStore} from '@/stores/favorites.ts'
import {useOnline} from '@/composables/useOnline.ts'
import {useRoutesApi} from '@/composables/useRoutesApi.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'
import type {Route, Stop} from '@/types/tranzy.ts'

export interface RecentItem {
  key: string
  recent: Recent
  plan?: FavoritePlan
  stop?: Stop
  route?: Route
}

export function useRecents(shown: number) {
  const favorites = useFavoritesStore()
  const {recents} = storeToRefs(favorites)
  const {routes} = useRoutesApi()
  const {stops} = useStopsApi()
  const {isOnline} = useOnline()

  return computed<RecentItem[]>(() => {
    const items: RecentItem[] = []
    for (const recent of recents.value) {
      const item: RecentItem = {key: recentKey(recent), recent}
      if ('stopId' in recent) item.stop = stops.value.find((s) => s.stop_id === recent.stopId)
      else if ('routeId' in recent) item.route = routes.value.find((r) => r.route_id === recent.routeId)
      else if (isOnline.value && !favorites.isPlanFavorite(recent.lat, recent.lon, recent.originLat, recent.originLon)) item.plan = recent
      if (item.stop || item.route || item.plan) items.push(item)
    }
    return items.slice(0, shown)
  })
}
