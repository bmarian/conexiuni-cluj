import {defineStore} from 'pinia'
import {ref} from 'vue'
import type {PlannedRoute} from '@/utils/trips.ts'

export const usePlannerStore = defineStore('planner', () => {
  const lastSelectedRouteKeys = ref<Record<string, string>>({})
  const plannedRoutes = ref<PlannedRoute[]>([])
  const currentQueryKey = ref<string | null>(null)

  function setSelectedRouteKey(queryKey: string, routeKey: string) {
    lastSelectedRouteKeys.value[queryKey] = routeKey
  }

  function getSelectedRouteKey(queryKey: string): string | null {
    return lastSelectedRouteKeys.value[queryKey] || null
  }

  return {
    lastSelectedRouteKeys,
    plannedRoutes,
    currentQueryKey,
    setSelectedRouteKey,
    getSelectedRouteKey
  }
})
