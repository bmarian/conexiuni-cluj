import {type RouteLocationRaw, useRoute, useRouter} from 'vue-router'

// Stops picked one after another on the map replace each other in the history.
export function useMapPicks() {
  const route = useRoute()
  const router = useRouter()
  const open = (target: RouteLocationRaw) => route.name === 'stop' || route.name === 'stops' ? router.replace(target) : router.push(target)
  return {
    openStop: (stopId: string) => open({name: 'stop', params: {stopId}}),
    openStops: (stopIds: string[]) => open({name: 'stops', params: {stopIds: stopIds.join(',')}}),
  }
}
