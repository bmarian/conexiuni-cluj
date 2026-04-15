import {defineStore} from 'pinia'
import {ref} from 'vue'
import type {ShapeInfo} from '@/types/tranzy.ts'

export const useRouteStore = defineStore('route', () => {
  const selectedShapeInfo = ref<ShapeInfo | null>(null)
  const selectedTripId = ref<string | null>(null)
  const fromStopId = ref<string | null>(null)
  const fromStopName = ref<string | null>(null)

  function setSelectedRoute(shapeInfo: ShapeInfo, tripId: string, stopId: string, stopName: string) {
    selectedShapeInfo.value = shapeInfo
    selectedTripId.value = tripId
    fromStopId.value = stopId
    fromStopName.value = stopName
  }

  return {selectedShapeInfo, selectedTripId, fromStopId, fromStopName, setSelectedRoute}
})
