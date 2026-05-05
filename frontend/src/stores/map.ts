import type {Shape, Vehicle} from "@/types/tranzy";
import type {RouteType} from "@/types/tranzy";
import {defineStore} from 'pinia'
import {ref} from "vue";
import {apiRequest} from "@/utils/request_cache.ts";

export type DisplayShape = {
  trip_id: string,
  route_short_name: string,
  route_long_name: string,
  route_color: string,
  route_type: RouteType,
}

export type HighlightedStop = { stopId: string; color: 'green' | 'purple' | 'red' | 'gray' | 'amber' }

export const useMapStore = defineStore('map', () => {
  const shapesToDisplay = ref<Array<[DisplayShape, Shape[]]>>([])
  const vehiclesToDisplay = ref<Vehicle[]>([])
  const highlightedStops = ref<HighlightedStop[]>([])
  const walkingPolylines = ref<[number, number][][]>([])
  const vehicleColor = ref<string | null>(null)
  const zoomOut = ref(false)
  const centerOnUser = ref(false)
  const flyToLocation = ref<{lat: number; lng: number} | null>(null)
  const pinnedLocation = ref<{lat: number; lng: number; label: string} | null>(null)
  const customOriginLocation = ref<{lat: number; lng: number; label: string} | null>(null)
  const pinnedLocationDragged = ref<{lat: number; lng: number} | null>(null)
  const customOriginLocationDragged = ref<{lat: number; lng: number} | null>(null)
  const drawerBottomPx = ref(0)
  const fitWalkingPolylines = ref(false)

  const setDrawerBottomPx = (px: number) => {
    drawerBottomPx.value = px
  }

  const setShapesToDisplay = async (displayShapes: DisplayShape[]) => {
    if (!displayShapes) return
    shapesToDisplay.value = await requestShapes(displayShapes)
  }

  const setLoadedShapes = (entries: Array<[DisplayShape, Shape[]]>) => {
    shapesToDisplay.value = entries
  }

  const requestShapes = async (displayShapes: DisplayShape[]): Promise<Array<[DisplayShape, Shape[]]>> => {
    if (!displayShapes?.length) return []

    const shapeIds = [...new Set(displayShapes.map(d => d.trip_id))].sort()
    const raw = (await apiRequest(`shapes?shape_ids=${shapeIds.join(',')}`) as Shape[]) ?? []

    const grouped = new Map<string, Shape[]>()
    for (const id of shapeIds) grouped.set(id, [])
    for (const pt of raw) {
      const bucket = grouped.get(pt.shape_id)
      if (bucket) bucket.push(pt)
    }
    return displayShapes.map((d): [DisplayShape, Shape[]] => [d, grouped.get(d.trip_id) ?? []])
  }

  const setWalkingPolylines = (polylines: [number, number][][]) => {
    walkingPolylines.value = polylines
  }

  const clearWalkingPolylines = () => {
    walkingPolylines.value = []
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

  const setFlyToLocation = (lat: number, lng: number) => {
    flyToLocation.value = {lat, lng}
  }

  const setPinnedLocation = (lat: number, lng: number, label: string) => {
    pinnedLocation.value = {lat, lng, label}
  }

  const clearPinnedLocation = () => {
    pinnedLocation.value = null
  }

  const setCustomOriginLocation = (lat: number, lng: number, label: string) => {
    customOriginLocation.value = {lat, lng, label}
  }

  const clearCustomOriginLocation = () => {
    customOriginLocation.value = null
  }

  const setPinnedLocationDragged = (lat: number, lng: number) => {
    pinnedLocationDragged.value = {lat, lng}
  }

  const clearPinnedLocationDragged = () => {
    pinnedLocationDragged.value = null
  }

  const setCustomOriginLocationDragged = (lat: number, lng: number) => {
    customOriginLocationDragged.value = {lat, lng}
  }

  const clearCustomOriginLocationDragged = () => {
    customOriginLocationDragged.value = null
  }

  return {
    centerOnUser,
    flyToLocation,
    pinnedLocation,
    customOriginLocation,
    pinnedLocationDragged,
    customOriginLocationDragged,
    shapesToDisplay,
    walkingPolylines,
    zoomOut,
    vehiclesToDisplay,
    highlightedStops,
    vehicleColor,

    setShapesToDisplay,
    setLoadedShapes,
    setVehiclesToDisplay,
    setHighlightedStops,
    setVehicleColor,
    setFlyToLocation,
    setPinnedLocation,
    clearPinnedLocation,
    setCustomOriginLocation,
    clearCustomOriginLocation,
    setPinnedLocationDragged,
    clearPinnedLocationDragged,
    setCustomOriginLocationDragged,
    clearCustomOriginLocationDragged,
    setWalkingPolylines,
    clearWalkingPolylines,
    drawerBottomPx,
    setDrawerBottomPx,
    fitWalkingPolylines,

    requestShapes,
  }
})
