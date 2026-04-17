<script setup lang="ts">
import {onMounted, onUnmounted, ref, shallowRef, watch} from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import {GeoSearchControl, OpenStreetMapProvider} from 'leaflet-geosearch'
import 'leaflet-geosearch/dist/geosearch.css'
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";
import {apiRequest} from "@/utils/request_cache.ts";
import type {Stop, Vehicle} from "@/types/tranzy.ts";
import {useRoute, useRouter} from "vue-router";
import {useI18n} from "vue-i18n";
import {type DisplayShape, type HighlightedStop, useMapStore} from "@/stores/map.ts";
import type {ShapePoint} from "@/types/map.ts";

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapStore = useMapStore()
const {shapesToDisplay, centerOnUser, zoomOut, vehiclesToDisplay, highlightedStops} = storeToRefs(mapStore)
const router = useRouter()
const route = useRoute()
const stopMarkers = new Map<string, L.Marker>()
let currentlyHighlightedStopId: string | null = null
const {t, locale} = useI18n()
const mapContainer = ref()

const map = shallowRef<L.Map>()
const stopGroup = shallowRef<L.FeatureGroup>()
const currentTileLayer = shallowRef<L.TileLayer>()
const userDot = shallowRef<L.Marker>()
const accuracyCircle = shallowRef<L.Circle>()
const shapeLayerGroup = shallowRef<L.FeatureGroup>()
const vehicleLayerGroup = shallowRef<L.FeatureGroup>()
const highlightedStopLayerGroup = shallowRef<L.FeatureGroup>()
const routeColorsCache = new Map<string | number, string>()

let isFirstLocationHandle = true
const DEFAULT_ZOOM = 16
const STOP_ZOOM_THRESHOLD = 16


const stopIcon = L.divIcon({
  className: 'bg-transparent border-none',
  html: `
      <div class="flex items-center justify-center w-6 h-6 rounded-full border-2 border-white shadow-sm z-20 bg-slate-500 dark:bg-slate-600 text-white">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-3.5 h-3.5">
          <path d="M4 16c0 .88.39 1.67 1 2.22V20c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1h8v1c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1.78c.61-.55 1-1.34 1-2.22V6c0-3.5-3.58-4-8-4s-8 .5-8 4v10zm3.5 1c-.83 0-1.5-.67-1.5-1.5S6.67 14 7.5 14s1.5.67 1.5 1.5S8.33 17 7.5 17zm9 0c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm-10-7V6h11v4H6.5z"/>
        </svg>
      </div>
    `,
  iconSize: [24, 24],
  iconAnchor: [12, 12],
  popupAnchor: [0, -12]
})

const selectedStopIcon = L.divIcon({
  className: 'bg-transparent border-none',
  html: `
    <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-lg z-50 bg-emerald-500 text-white animate-bounce">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-4 h-4">
        <path d="M4 16c0 .88.39 1.67 1 2.22V20c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1h8v1c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1.78c.61-.55 1-1.34 1-2.22V6c0-3.5-3.58-4-8-4s-8 .5-8 4v10zm3.5 1c-.83 0-1.5-.67-1.5-1.5S6.67 14 7.5 14s1.5.67 1.5 1.5S8.33 17 7.5 17zm9 0c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm-10-7V6h11v4H6.5z"/>
      </svg>
    </div>
  `,
  iconSize: [32, 32], iconAnchor: [16, 16], popupAnchor: [0, -16]
})

const highlightSelectedStop = (stopId?: string) => {
  if (currentlyHighlightedStopId && stopMarkers.has(currentlyHighlightedStopId)) {
    const oldMarker = stopMarkers.get(currentlyHighlightedStopId)!
    oldMarker.setIcon(stopIcon)
    oldMarker.setZIndexOffset(0)
    if (stopGroup.value && !stopGroup.value.hasLayer(oldMarker)) stopGroup.value.addLayer(oldMarker)
  }
  if (stopId && stopMarkers.has(stopId)) {
    const newMarker = stopMarkers.get(stopId)!
    newMarker.setIcon(selectedStopIcon)
    newMarker.setZIndexOffset(1000)
    if (stopGroup.value && stopGroup.value.hasLayer(newMarker)) stopGroup.value.removeLayer(newMarker)
    if (map.value && !map.value.hasLayer(newMarker)) newMarker.addTo(map.value)
    currentlyHighlightedStopId = stopId
  }
}

const handleZoomVisibility = () => {
  if (!map.value || !stopGroup.value) return

  if (map.value.getZoom() >= STOP_ZOOM_THRESHOLD) {
    if (!map.value.hasLayer(stopGroup.value)) {
      map.value.addLayer(stopGroup.value)
    }
  } else {
    if (map.value.hasLayer(stopGroup.value)) {
      map.value.removeLayer(stopGroup.value)
    }
  }
}

const mapInit = (lat: number, lon: number, zoom: number) => {
  map.value = L.map(mapContainer.value).setView([lat, lon], zoom)
  map.value.on('locationfound', updateLiveLocation)
  map.value.on('locationerror', (e) => {
    console.warn("GPS Error:", e.message)
    userStore.setHasLocationPermission(false)
  })

  currentTileLayer.value = L.tileLayer(`https://{s}.basemaps.cartocdn.com/${isDarkMode.value ? 'dark_all' : 'light_all'}/{z}/{x}/{y}{r}.png`, {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>, &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20
  }).addTo(map.value)

  const centerControl = new L.Control({position: 'topleft'})
  centerControl.onAdd = () => {
    const container = L.DomUtil.create('div', 'leaflet-bar leaflet-control')
    const button = L.DomUtil.create('a', 'flex! items-center justify-center border-b-0!', container)

    button.href = '#'
    button.title = t('Center')
    button.setAttribute('role', 'button')

    button.innerHTML = `
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-[18px] h-[18px] text-slate-700 dark:text-slate-300">
        <circle cx="12" cy="12" r="4"/>
        <path d="M12 2v2"/>
        <path d="M12 20v2"/>
        <path d="M5 12H2"/>
        <path d="M22 12h-3"/>
      </svg>
    `

    L.DomEvent.disableClickPropagation(container)
    L.DomEvent.on(button, 'click', (e) => {
      e.preventDefault()
      const location = userDot.value?.getLatLng()
      if (!location || !map.value) return

      map.value.flyTo(location, DEFAULT_ZOOM, {duration: 1})
    })

    return container
  }

  map.value.addControl(centerControl)

  const CLUJ_VIEWBOX = '23.50,46.81,23.70,46.71'
  const provider = new OpenStreetMapProvider({
    params: {
      countrycodes: 'ro',
      viewbox: CLUJ_VIEWBOX,
      bounded: 0
    }
  })
  const customSearchIcon = L.divIcon({
    className: 'bg-transparent! border-none!',
    html: `
    <div class="w-8 h-8 drop-shadow-lg -mt-2 text-blue-500 dark:text-blue-400">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor">
        <path fill-rule="evenodd" d="M11.54 22.351l.07.04.028.016a.76.76 0 00.723 0l.028-.015.071-.041a16.975 16.975 0 001.144-.742 19.58 19.58 0 002.683-2.282c1.944-1.99 3.963-4.98 3.963-8.827a8.25 8.25 0 00-16.5 0c0 3.846 2.02 6.837 3.963 8.827a19.58 19.58 0 002.682 2.282 16.975 16.975 0 001.145.742zM12 13.5a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" />
      </svg>
    </div>
  `,
    iconSize: [24, 24],
    iconAnchor: [12, 24]
  })
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-expect-error
  const searchControl = new GeoSearchControl({
    provider,
    showMarker: true,
    marker: {
      icon: customSearchIcon,
      draggable: false,
    },
    retainZoomLevel: false,
    animateZoom: true,
    autoClose: true,
    searchLabel: t('Search location...'),
    keepResult: true
  })
  map.value.addControl(searchControl)

  map.value.locate({
    watch: true,
    enableHighAccuracy: false,
    maximumAge: 2000
  })
  shapeLayerGroup.value = L.featureGroup().addTo(map.value)

  stopGroup.value = L.featureGroup()
  map.value.on('zoomend', handleZoomVisibility)
  shapeLayerGroup.value = L.featureGroup().addTo(map.value)
  vehicleLayerGroup.value = L.featureGroup().addTo(map.value)
  highlightedStopLayerGroup.value = L.featureGroup().addTo(map.value)
  handleZoomVisibility()
}

const stopsInit = async () => {
  const stops = await apiRequest('stops') as Stop[]
  if (!Array.isArray(stops) || !stops.length || !stopGroup.value) return

  for (let i = 0; i < stops.length; i++) {
    const {stop_lat, stop_lon, stop_name, stop_id} = stops[i]!

    const marker = L.marker([stop_lat, stop_lon], {icon: stopIcon})
    marker.once('click', (e) => {
      const popupContent = `<span>${stop_name}</span>`
      e.target.bindPopup(popupContent).openPopup()
    })
    marker.on('click', () => {
      router.push({name: 'stop', params: {stopId: stop_id}, replace: true})
    })

    marker.addTo(stopGroup.value)
    stopMarkers.set(stop_id.toString(), marker)
  }
  highlightSelectedStop(route.params.stopId as string)
}

const blueDotIcon = L.divIcon({
  className: 'bg-transparent border-none',
  html: `
    <div class="relative flex items-center justify-center w-4 h-4">
      <div class="absolute w-4 h-4 bg-blue-500 border-2 border-white rounded-full shadow-md z-10"></div>
      <div class="absolute w-full h-full bg-blue-400 rounded-full animate-ping opacity-75"></div>
    </div>
  `,
  iconSize: [16, 16],
  iconAnchor: [8, 8]
})

const updateLiveLocation = (e: L.LocationEvent) => {
  if (!map.value) return

  userStore.setUserLocation(e.latlng.lat, e.latlng.lng)
  if (isFirstLocationHandle) {
    map.value.flyTo(e.latlng, DEFAULT_ZOOM, {duration: 1})
    isFirstLocationHandle = false
  }

  if (userDot.value && accuracyCircle.value) {
    userDot.value.setLatLng(e.latlng)

    accuracyCircle.value.setLatLng(e.latlng)
    accuracyCircle.value.setRadius(e.accuracy)
  } else {
    accuracyCircle.value = L.circle(e.latlng, {
      radius: e.accuracy,
      color: '#3b82f6',
      fillColor: '#3b82f6',
      fillOpacity: 0.15,
      weight: 1,
      interactive: false
    }).addTo(map.value)

    userDot.value = L.marker(e.latlng, {
      icon: blueDotIcon,
      interactive: false
    }).addTo(map.value)
  }
}

onMounted(() => {
  mapInit(46.7712, 23.6236, DEFAULT_ZOOM)
  void stopsInit()
})

watch(() => route.params.stopId, (newId) => {
  highlightSelectedStop(newId as string)
})

watch(isDarkMode, (newValue) => {
  if (!currentTileLayer.value) return

  const newUrl = `https://{s}.basemaps.cartocdn.com/${newValue ? 'dark_all' : 'light_all'}/{z}/{x}/{y}{r}.png`
  currentTileLayer.value.setUrl(newUrl)
})

watch(locale, () => {
  const searchInput = document.querySelector('.leaflet-control-geosearch input.glass') as HTMLInputElement

  if (searchInput) {
    searchInput.placeholder = t('Search location...')
  }
})

watch(shapesToDisplay, (newShapes) => {
  if (!shapeLayerGroup.value || !map.value) return
  shapeLayerGroup.value.clearLayers()
  const drawnPaths = new Set<string>()
  const duplicateDashPatterns = ['', '8 7', '2 7', '10 4 2 4', '1 6']

  const groupedStarts = new Map<string, {
    lat: number,
    lng: number,
    routes: { name: string, color: string }[]
  }>()

  const hasHighlights = highlightedStops.value.length > 0
  const colorCounts = new Map<string, number>()

  for (let i = 0; i < newShapes.length; i++) {
    const [displayShape, shapeData]: [DisplayShape, ShapePoint[]] = newShapes[i]!
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue
    const routeColor = displayShape.route_color
    colorCounts.set(routeColor, (colorCounts.get(routeColor) ?? 0) + 1)
  }
  const colorSeen = new Map<string, number>()

  for (let i = 0; i < newShapes.length; i++) {
    const [displayShape, shapeData]: [DisplayShape, ShapePoint[]] = newShapes[i]!
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue

    const latLngs: L.LatLngTuple[] = shapeData.map(sd => [sd.shape_pt_lat, sd.shape_pt_lon])
    const routeColor = displayShape.route_color
    routeColorsCache.set(displayShape.trip_id, routeColor)

    const colorVariantIdx = colorSeen.get(routeColor) ?? 0
    colorSeen.set(routeColor, colorVariantIdx + 1)
    const hasDuplicateColor = (colorCounts.get(routeColor) ?? 0) > 1
    const dashArray = hasDuplicateColor
      ? duplicateDashPatterns[colorVariantIdx % duplicateDashPatterns.length]
      : ''

    const pathSignature = latLngs.map(ll => `${ll[0].toFixed(4)},${ll[1].toFixed(4)}`).join('|')
    const signature = `${dashArray}|${pathSignature}`
    if (drawnPaths.has(signature)) continue
    drawnPaths.add(signature)

    L.polyline(latLngs, {
      color: '#94a3b8',
      weight: 5,
      opacity: 0.7,
      dashArray: dashArray || undefined,
      smoothFactor: 1.5,
      lineJoin: 'round',
      lineCap: 'round'
    }).addTo(shapeLayerGroup.value!)

    if (!hasHighlights) {
      const startPoint = latLngs[0]
      if (startPoint && displayShape.route_short_name) {
        const key = `${startPoint[0].toFixed(4)},${startPoint[1].toFixed(4)}`
        if (!groupedStarts.has(key)) groupedStarts.set(key, {lat: startPoint[0], lng: startPoint[1], routes: []})
        const existing = groupedStarts.get(key)!
        if (!existing.routes.some(r => r.name === displayShape.route_short_name)) {
          existing.routes.push({name: displayShape.route_short_name, color: routeColor})
        }
      }

      const endPoint = latLngs[latLngs.length - 1]
      if (endPoint) {
        const endMarkerIcon = L.divIcon({
          className: 'bg-transparent border-none !overflow-visible',
          html: `
            <div class="flex items-center justify-center w-6 h-6 rounded-full border-[3px] border-white dark:border-[#0f172a] shadow-md z-20"
                 style="background-color: ${routeColor};">
              <div class="w-1.5 h-1.5 bg-white dark:bg-[#0f172a] rounded-[2px]"></div>
            </div>
          `,
          iconSize: [24, 24], iconAnchor: [12, 12]
        })
        L.marker(endPoint, {icon: endMarkerIcon}).addTo(shapeLayerGroup.value!)
      }
    }
  }

  if (!hasHighlights) {
    groupedStarts.forEach((data) => {
      const routesHtml = data.routes.map(r =>
        `<div style="background-color: ${r.color};"
              class="flex items-center justify-center min-w-[28px] h-[28px] px-2 rounded-full text-white text-[11px] font-black shadow-md border-[3px] border-white dark:border-[#0f172a]">
          ${r.name}
        </div>`
      ).join('')
      const startMarkerIcon = L.divIcon({
        className: 'bg-transparent border-none !overflow-visible',
        html: `<div class="absolute flex flex-row items-center justify-center gap-0.5 whitespace-nowrap" style="transform: translate(-50%, -50%);">${routesHtml}</div>`,
        iconSize: [0, 0], iconAnchor: [0, 0]
      })
      L.marker([data.lat, data.lng], {icon: startMarkerIcon}).addTo(shapeLayerGroup.value!)
    })
  }

  if (zoomOut.value && shapeLayerGroup.value.getLayers().length > 0) {
    map.value.fitBounds(shapeLayerGroup.value.getBounds(), {padding: [50, 50]})
    zoomOut.value = false
  }
}, {deep: true})

const BUS_STOP_PATH = 'M4 16c0 .88.39 1.67 1 2.22V20c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1h8v1c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1.78c.61-.55 1-1.34 1-2.22V6c0-3.5-3.58-4-8-4s-8 .5-8 4v10zm3.5 1c-.83 0-1.5-.67-1.5-1.5S6.67 14 7.5 14s1.5.67 1.5 1.5S8.33 17 7.5 17zm9 0c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm-10-7V6h11v4H6.5z'

const makeHighlightIcon = (color: 'green' | 'purple' | 'red' | 'gray') => {
  const bg = color === 'green' ? '#10b981' : color === 'purple' ? '#a855f7' : color === 'red' ? '#f43f5e' : '#64748b'
  return L.divIcon({
    className: 'bg-transparent border-none !overflow-visible',
    html: `<div style="width:24px;height:24px;border-radius:50%;background:${bg};border:2px solid white;box-shadow:0 1px 4px rgba(0,0,0,0.22);display:flex;align-items:center;justify-content:center;">
      <svg viewBox="0 0 24 24" fill="white" width="14" height="14"><path d="${BUS_STOP_PATH}"/></svg>
    </div>`,
    iconSize: [24, 24],
    iconAnchor: [12, 12],
  })
}

watch(highlightedStops, (stops: HighlightedStop[]) => {
  if (!highlightedStopLayerGroup.value) return
  highlightedStopLayerGroup.value.clearLayers()
  for (const {stopId, color} of stops) {
    const marker = stopMarkers.get(stopId)
    if (!marker) continue
    const latlng = marker.getLatLng()
    L.marker(latlng, {
      icon: makeHighlightIcon(color),
      zIndexOffset: color === 'green' ? 1200 : color === 'purple' ? 1100 : color === 'red' ? 1000 : 800,
      interactive: false,
    }).addTo(highlightedStopLayerGroup.value!)
  }
}, {deep: true})

watch(vehiclesToDisplay, (vehicles) => {
  if (!vehicleLayerGroup.value || !map.value) return
  vehicleLayerGroup.value.clearLayers()

  for (let i = 0; i < vehicles.length; i++) {
    const vehicle = vehicles[i]! as Vehicle & { route_short_name: string, route_color?: string, heading: number }

    if (vehicle.latitude <= 0 || vehicle.longitude <= 0) continue

    const resolvedColor = routeColorsCache.get(vehicle.trip_id) || vehicle.route_color
    if (!resolvedColor) continue
    const routeName = vehicle.route_short_name || ''
    const titleText = routeName ? `${routeName} • ${vehicle.label}` : vehicle.label
    const busIcon = L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `
        <div class="relative flex items-center">
          <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-md z-30 shrink-0"
               style="background-color: ${resolvedColor};">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white" class="w-4 h-4"
                 style="transform: rotate(${vehicle.heading || 0}deg);">
              <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="currentColor" stroke-width="1" stroke-linejoin="round"/>
            </svg>
          </div>
          <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md flex flex-col whitespace-nowrap z-20 pointer-events-none">
            <span class="font-bold text-sm tracking-wide">${titleText}</span>
            <span class="text-xs text-slate-400">${Math.round(vehicle.speed)} km/h</span>
          </div>
        </div>
      `,
      iconSize: [32, 32],
      iconAnchor: [16, 16],
    })

    const marker = L.marker([vehicle.latitude, vehicle.longitude], {
      icon: busIcon,
      zIndexOffset: 1000
    })

    marker.addTo(vehicleLayerGroup.value!)
  }
}, {deep: true})

watch(centerOnUser, (shouldCenter) => {
  if (!shouldCenter) return

  const userLocation = userDot.value?.getLatLng()
  if (!userLocation) return

  const mapValue = map.value
  if (!mapValue) return

  mapValue.flyTo(userLocation, DEFAULT_ZOOM, {duration: 1})
  centerOnUser.value = false
})

onUnmounted(() => {
  if (map.value) {
    map.value.stopLocate()
    map.value.off('locationfound', updateLiveLocation)
    map.value.off('zoomend', handleZoomVisibility)

    map.value.remove()
  }
})
</script>

<template>
  <div ref="mapContainer" class="map-container"></div>
</template>
