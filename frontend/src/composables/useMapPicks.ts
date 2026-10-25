import {type RouteLocationRaw, useRoute, useRouter} from 'vue-router'
import {useMapStore} from '@/stores/map.ts'
import {stopLocation, stopsPath} from '@/utils/pagePaths.ts'

const PICKED = new Set(['stop', 'stop-id', 'stops', 'stops-id'])

// Stops picked one after another on the map replace each other in the history.
export function useMapPicks() {
  const route = useRoute()
  const router = useRouter()
  const mapStore = useMapStore()
  const open = (target: RouteLocationRaw) => {
    mapStore.revealDrawer = true
    return PICKED.has(String(route.name)) ? router.replace(target) : router.push(target)
  }
  return {
    openStop: (stopId: string) => open(stopLocation(stopId)),
    openStops: (stopIds: string[]) => open(stopsPath(stopIds)),
  }
}
