import {ref} from 'vue'
import {useRouter} from 'vue-router'
import {useRouteStore} from '@/stores/route.ts'
import {useMapStore} from '@/stores/map.ts'
import {useRouteShapeInfoApi} from '@/composables/useRouteShapeInfoApi.ts'
import {linePath} from '@/utils/pagePaths.ts'
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Route, type RouteDirection} from '@/types/tranzy.ts'

export function useOpenRoute() {
  const router = useRouter()
  const routeStore = useRouteStore()
  const mapStore = useMapStore()
  const {fetchShapeInfo} = useRouteShapeInfoApi()
  const openingRouteId = ref<number | null>(null)

  async function openRoute(route: Route, direction?: RouteDirection) {
    if (openingRouteId.value === route.route_id) return
    openingRouteId.value = route.route_id
    mapStore.clearPinnedLocation()
    try {
      const shapeInfo = await fetchShapeInfo(route)
      const tripId = `${route.route_id}${direction === '1' ? INCOMING_SUFFIX : OUTGOING_SUFFIX}`
      routeStore.setSelectedRoute(shapeInfo, tripId, '', '')
      await router.push(linePath(route.route_id, direction ?? 'auto'))
    } catch (e) {
      console.error('Failed to load route:', e)
    } finally {
      openingRouteId.value = null
    }
  }

  return {openingRouteId, openRoute}
}
