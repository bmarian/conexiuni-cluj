import {type RouteLocationRaw, useRoute, useRouter} from 'vue-router'
import {useMapStore} from '@/stores/map.ts'
import {stopLocation} from '@/utils/pagePaths.ts'

// Stops picked one after another on the map replace each other in the history.
export function useMapPicks() {
  const route = useRoute()
  const router = useRouter()
  const mapStore = useMapStore()
  const open = (target: RouteLocationRaw) => {
    mapStore.revealDrawer = true
    return route.name === 'stop' || route.name === 'stop-id' || route.name === 'stops' ? router.replace(target) : router.push(target)
  }
  return {
    openStop: (stopId: string) => open(stopLocation(stopId)),
    openStops: (stopIds: string[]) => open({name: 'stops', params: {stopIds: stopIds.join(',')}}),
  }
}
