import type {Shape, Vehicle} from "@/types/tranzy";
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

export type HighlightedStop = { stopId: string; color: 'green' | 'purple' | 'gray' }

export const useMapStore = defineStore('map', () => {
  const shapesToDisplay = ref<Array<[DisplayShape, Shape[]]>>([])
  const vehiclesToDisplay = ref<Vehicle[]>([])
  const highlightedStops = ref<HighlightedStop[]>([])
  const vehicleColor = ref<string | null>(null)
  const zoomOut = ref(false)
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

  const setVehiclesToDisplay = (vehicles: Vehicle[]) => {
    if (!vehicles) return
    vehiclesToDisplay.value = vehicles
  }

  const setHighlightedStops = (stops: HighlightedStop[]) => {
    highlightedStops.value = stops
  }

  const setVehicleColor = (color: string | null) => {
    vehicleColor.value = color
  }

  return {
    centerOnUser,
    shapesToDisplay,
    zoomOut,
    vehiclesToDisplay,
    highlightedStops,
    vehicleColor,

    setShapesToDisplay,
    setVehiclesToDisplay,
    setHighlightedStops,
    setVehicleColor,

    requestShapes,
  }
})
