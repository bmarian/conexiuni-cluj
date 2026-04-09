import type {Shape} from "@/types/tranzy";
import {defineStore} from 'pinia'
import {ref} from "vue";
import {apiRequest} from "@/utils/request_cache.ts";

export type DisplayShape = {
  trip_id: string,
  route_short_name: string,
  route_long_name: string,
  route_color: string,
  route_type: number,
}

export const useMapStore = defineStore('map', () => {
  const shapesToDisplay = ref<Array<[DisplayShape, Shape[]]>>([])
  const centerOnUser = ref(false)

  const setShapesToDisplay = async (displayShapes: DisplayShape[]) => {
    if (!displayShapes) return
    shapesToDisplay.value = await requestShapes(displayShapes)
  }

  const requestShapes = async (displayShapes: DisplayShape[]) => {
    if (!displayShapes) return []

    const results: Array<[DisplayShape, Shape[]]> = []
    for (let i = 0; i < displayShapes.length; i++) {
      const displayShape = displayShapes[i]!
      const response = await apiRequest(`shapes?shape_id=${displayShape.trip_id}`) as Shape[]
      results.push([displayShape, response])
    }

    return results
  }

  return {
    centerOnUser,
    shapesToDisplay,

    setShapesToDisplay,
    requestShapes,
  }
})
