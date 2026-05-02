import {defineStore} from 'pinia'
import {ref} from 'vue'

export const usePlannerStore = defineStore('planner', () => {
  const lastSelectedRouteKeys = ref<Record<string, string>>({})

  function setSelectedRouteKey(queryKey: string, routeKey: string) {
    lastSelectedRouteKeys.value[queryKey] = routeKey
  }

  function getSelectedRouteKey(queryKey: string): string | null {
    return lastSelectedRouteKeys.value[queryKey] || null
  }

  return {lastSelectedRouteKeys, setSelectedRouteKey, getSelectedRouteKey}
})
