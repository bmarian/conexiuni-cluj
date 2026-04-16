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

  /** Synchronous setter for callers that already have shape data in memory
   *  (e.g. RouteView preloading both directions). Avoids the network round-trip
   *  through `setShapesToDisplay` and the brief flicker that comes with it. */
  const setLoadedShapes = (entries: Array<[DisplayShape, Shape[]]>) => {
    shapesToDisplay.value = entries
  }

  const requestShapes = async (displayShapes: DisplayShape[]) => {
    if (!displayShapes) return []

    return Promise.all(
      displayShapes.map(async (displayShape): Promise<[DisplayShape, Shape[]]> => {
        const response = (await apiRequest(`shapes?shape_id=${displayShape.trip_id}`) as Shape[]) ?? []
        return [displayShape, response]
      }),
    )
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
    setLoadedShapes,
    setVehiclesToDisplay,
    setHighlightedStops,
    setVehicleColor,

    requestShapes,
  }
})
