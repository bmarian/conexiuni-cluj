import {cityDateTime} from '@/utils/cityClock.ts'
import {defineStore} from 'pinia'
import {ref} from 'vue'

export type PlanTimeMode = 'now' | 'leave' | 'arrive' | 'last'

export const usePlannerStore = defineStore('planner', () => {
  const lastSelectedRouteKeys = ref<Record<string, string>>({})
  const currentQueryKey = ref<string | null>(null)

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
    if (mode !== 'now' && mode !== 'last' && !timeValue.value) {
      timeValue.value = cityDateTime(new Date())
    }
  }

  function setTimeValue(v: string) {
    timeValue.value = v
  }

  return {
    lastSelectedRouteKeys,
    currentQueryKey,
    timeMode,
    timeValue,
    setSelectedRouteKey,
    getSelectedRouteKey,
    setTimeMode,
    setTimeValue,
  }
})
