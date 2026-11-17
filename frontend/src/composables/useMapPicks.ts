import {type RouteLocationRaw, useRoute, useRouter} from 'vue-router'
import {GROUP_BUILDER} from '@/composables/useGroupBuilder.ts'
import {useMapStore} from '@/stores/map.ts'
import {stopLocation, stopsPath} from '@/utils/pagePaths.ts'

const PICKED = new Set(['stop', 'stop-id', 'stops', 'stops-id'])

// Stops picked one after another on the map replace each other in the history.
export function useMapPicks() {
  const route = useRoute()
  const router = useRouter()
  const mapStore = useMapStore()
  const open = async (target: RouteLocationRaw) => {
    if (GROUP_BUILDER.has(String(route.name))) return
    mapStore.revealDrawer = true
    return PICKED.has(String(route.name)) ? router.replace(target) : router.push(target)
  }
  return {
    openStop: (stopId: string) => open(stopLocation(stopId)),
    openStops: (stopIds: string[]) => open(stopsPath(stopIds)),
  }
}
