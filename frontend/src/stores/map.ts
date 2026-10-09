import type {Shape, Vehicle} from "@/types/tranzy";
import type {RouteType} from "@/types/tranzy";
import {defineStore} from 'pinia'
import {ref} from "vue";
import {apiRequest} from "@/utils/api.ts";
import {decodePolyline, network} from "@/utils/network.ts";

const shapeCache = new Map<string, Promise<Shape[]>>()
const shapeKey = (tripId: string) => `${tripId}@${network.value?.shapeHash(tripId) ?? ''}`

async function fetchShapes(tripIds: string[]): Promise<Map<string, Shape[]>> {
  const missing = tripIds.filter((id) => !shapeCache.has(shapeKey(id)))
  if (missing.length) {
    const request = apiRequest(`shapes?format=polyline&shape_ids=${missing.join(',')}`) as Promise<Record<string, string>>
    for (const id of missing) {
      const key = shapeKey(id)
      const points = request.then((encoded) => decodePolyline(encoded?.[id] ?? '').map(([lat, lon], i): Shape => ({
        shape_id: id, shape_pt_lat: lat, shape_pt_lon: lon, shape_pt_sequence: i, shape_dist_traveled: -1,
      })))
      shapeCache.set(key, points)
      points.then((p) => { if (!p.length) shapeCache.delete(key) }, () => shapeCache.delete(key))
    }
  }
  const out = new Map<string, Shape[]>()
  await Promise.all(tripIds.map(async (id) => out.set(id, await shapeCache.get(shapeKey(id))!)))
  return out
}

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
  const directionArrowAtStart = ref(false)
  const centerOnUser = ref(false)
  const flyToLocation = ref<{lat: number; lng: number} | null>(null)
  const pinnedLocation = ref<{lat: number; lng: number; label: string} | null>(null)
  const customOriginLocation = ref<{lat: number; lng: number; label: string} | null>(null)
  const pinnedLocationDragged = ref<{lat: number; lng: number} | null>(null)
  const customOriginLocationDragged = ref<{lat: number; lng: number} | null>(null)
  const drawerBottomPx = ref(0)
  const drawerRightPx = ref(0)
  const fitWalkingPolylines = ref(false)

  const setDrawerBottomPx = (px: number) => {
    drawerBottomPx.value = px
  }

  const setDrawerRightPx = (px: number) => {
    drawerRightPx.value = px
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
    const grouped = await fetchShapes(shapeIds)
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
    directionArrowAtStart,

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
    drawerRightPx,
    setDrawerRightPx,
    fitWalkingPolylines,

    requestShapes,
  }
})
