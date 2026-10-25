import {computed} from 'vue'
import {useRouter, type RouteLocationNormalizedLoaded} from 'vue-router'

// The route while it is this view's own; a kept-alive view leaving keeps the last one instead of the next page's.
export function useOwnRoute(name: string) {
  const router = useRouter()
  return computed<RouteLocationNormalizedLoaded>((prev) => (!prev || router.currentRoute.value.name === name ? router.currentRoute.value : prev))
}
