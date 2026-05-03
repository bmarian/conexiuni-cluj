import {defineStore} from 'pinia'
import {ref} from 'vue'
import type {PlannedRoute} from '@/utils/trips.ts'

export type PlanTimeMode = 'now' | 'leave' | 'arrive'

export const usePlannerStore = defineStore('planner', () => {
  const lastSelectedRouteKeys = ref<Record<string, string>>({})
  const plannedRoutes = ref<PlannedRoute[]>([])
  const currentQueryKey = ref<string | null>(null)

  // Google-style departure time filter — kept across navigations within the session.
  const timeMode = ref<PlanTimeMode>('now')
  // Local datetime in the form "YYYY-MM-DDTHH:mm" (matches <input type="datetime-local">).
  const timeValue = ref<string>('')

  function setSelectedRouteKey(queryKey: string, routeKey: string) {
    lastSelectedRouteKeys.value[queryKey] = routeKey
  }

  function getSelectedRouteKey(queryKey: string): string | null {
    return lastSelectedRouteKeys.value[queryKey] || null
  }

  function setTimeMode(mode: PlanTimeMode) {
    timeMode.value = mode
    if (mode !== 'now' && !timeValue.value) {
      // Default to current local time, rounded to the next minute.
      const now = new Date()
      const pad = (n: number) => String(n).padStart(2, '0')
      timeValue.value = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T${pad(now.getHours())}:${pad(now.getMinutes())}`
    }
  }

  function setTimeValue(v: string) {
    timeValue.value = v
  }

  return {
    lastSelectedRouteKeys,
    plannedRoutes,
    currentQueryKey,
    timeMode,
    timeValue,
    setSelectedRouteKey,
    getSelectedRouteKey,
    setTimeMode,
    setTimeValue
  }
})
