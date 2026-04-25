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
import {useSettingsStore} from "@/stores/settings.ts";

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapStore = useMapStore()
const settingsStore = useSettingsStore()
const {shapesToDisplay, centerOnUser, zoomOut, vehiclesToDisplay, highlightedStops} = storeToRefs(mapStore)
const {easterEggActive} = storeToRefs(settingsStore)
const router = useRouter()
const route = useRoute()
const stopMarkers = new Map<string, L.Marker>()
const stopNames = new Map<string, string>()
const currentlyHighlightedStopId = ref<string | null>(null)
const selectedStopVehicleId = ref<number | null>(null)
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
let resizeRaf = 0
let resizeTimer: ReturnType<typeof setTimeout> | null = null
const DEFAULT_ZOOM = 16
const STOP_ZOOM_THRESHOLD = 16
const CLUJ_COUNTY_SW: L.LatLngTuple = [46.38, 22.75]
const CLUJ_COUNTY_NE: L.LatLngTuple = [47.50, 24.27]
const CLUJ_COUNTY_BOUNDS: L.LatLngBoundsLiteral = [CLUJ_COUNTY_SW, CLUJ_COUNTY_NE]
const MIN_ZOOM = 9
const CLUJ_VIEWBOX = '23.50,46.81,23.70,46.71'
const TILE_LAYER_ATTRIBUTION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>, &copy; <a href="https://carto.com/attributions">CARTO</a> | &copy; <a href="https://tranzy.ai/" target="_blank" rel="noopener">tranzy.ai</a>, &copy; <a href="https://ctpcj.ro" target="_blank" rel="noopener">CTP Cluj-Napoca</a>'
const DUPLICATE_DASH_PATTERNS = ['', '8 7', '2 7', '10 4 2 4', '1 6']

type DisplayVehicle = Vehicle & { route_short_name: string, route_color?: string, heading: number }
type ShapeLayerEntry = [DisplayShape, ShapePoint[]]
type GroupedStart = {
  lat: number
  lng: number
  routes: { name: string, color: string }[]
}

const getTileLayerUrl = (useDarkMode: boolean) => {
  return `https://{s}.basemaps.cartocdn.com/${useDarkMode ? 'dark_all' : 'light_all'}/{z}/{x}/{y}{r}.png`
}


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

const makeSelectedStopIcon = () => {
  if (easterEggActive.value) {
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div class="animate-bounce" style="width:28px;height:36px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="26" height="34" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="#22c55e"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.4" cy="8.5" r="0.6" fill="#15803d"/>
          <circle cx="8.4" cy="8.5" r="0.6" fill="#15803d"/>
        </svg>
      </div>`,
      iconSize: [28, 36], iconAnchor: [14, 34], popupAnchor: [0, -34]
    })
  }
  return L.divIcon({
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
}

const highlightSelectedStop = (stopId?: string) => {
  if (currentlyHighlightedStopId.value && stopMarkers.has(currentlyHighlightedStopId.value)) {
    const oldMarker = stopMarkers.get(currentlyHighlightedStopId.value)!
    oldMarker.setIcon(stopIcon)
    oldMarker.setZIndexOffset(0)
    if (map.value && map.value.hasLayer(oldMarker)) map.value.removeLayer(oldMarker)
    if (stopGroup.value && !stopGroup.value.hasLayer(oldMarker)) stopGroup.value.addLayer(oldMarker)
  }
  currentlyHighlightedStopId.value = null
  if (stopId && stopMarkers.has(stopId)) {
    const newMarker = stopMarkers.get(stopId)!
    newMarker.setIcon(makeSelectedStopIcon())
    newMarker.setZIndexOffset(1000)
    if (stopGroup.value && stopGroup.value.hasLayer(newMarker)) stopGroup.value.removeLayer(newMarker)
    if (map.value && !map.value.hasLayer(newMarker)) newMarker.addTo(map.value)
    currentlyHighlightedStopId.value = stopId
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

const invalidateMapSize = () => {
  if (!map.value) return
  map.value.invalidateSize({pan: false, animate: false})
}

const scheduleInvalidateMapSize = () => {
  if (resizeRaf) cancelAnimationFrame(resizeRaf)
  resizeRaf = requestAnimationFrame(() => {
    invalidateMapSize()
    resizeRaf = 0
  })

  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    invalidateMapSize()
  }, 280)
}

const createCenterControl = (mapValue: L.Map) => {
  const centerControl = new L.Control({position: 'topleft'})
  centerControl.onAdd = () => {
    const container = L.DomUtil.create('div', 'leaflet-bar leaflet-control')
    const button = L.DomUtil.create('a', 'flex! items-center justify-center', container)

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
      if (!location) return

      mapValue.flyTo(location, DEFAULT_ZOOM, {duration: 1})
    })

    return container
  }

  return centerControl
}

const createSearchControl = () => {
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
  return new GeoSearchControl({
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
}

const initLayerGroups = (mapValue: L.Map) => {
  stopGroup.value = L.featureGroup()
  shapeLayerGroup.value = L.featureGroup().addTo(mapValue)
  vehicleLayerGroup.value = L.featureGroup().addTo(mapValue)
  highlightedStopLayerGroup.value = L.featureGroup().addTo(mapValue)
  mapValue.on('zoomend', handleZoomVisibility)
  handleZoomVisibility()
}

const mapInit = (lat: number, lon: number, zoom: number) => {
  const mapValue = L.map(mapContainer.value, {
    maxBounds: CLUJ_COUNTY_BOUNDS,
    maxBoundsViscosity: 1.0,
    minZoom: MIN_ZOOM,
  }).setView([lat, lon], zoom)

  map.value = mapValue
  mapValue.on('locationfound', updateLiveLocation)
  mapValue.on('click', () => { selectedStopVehicleId.value = null })
  mapValue.on('locationerror', (e) => {
    console.warn("GPS Error:", e.message)
    userStore.setHasLocationPermission(false)
  })

  currentTileLayer.value = L.tileLayer(getTileLayerUrl(isDarkMode.value), {
    attribution: TILE_LAYER_ATTRIBUTION,
    maxZoom: 20,
    minZoom: MIN_ZOOM,
    bounds: CLUJ_COUNTY_BOUNDS,
  }).addTo(mapValue)

  mapValue.addControl(createCenterControl(mapValue))
  mapValue.addControl(createSearchControl())
  mapValue.locate({
    watch: true,
    enableHighAccuracy: false,
    maximumAge: 30000,
    timeout: 10000
  })

  initLayerGroups(mapValue)
  mapContainer.value?.classList.toggle('hungry-theme', easterEggActive.value)
}

const stopsInit = async () => {
  const stops = await apiRequest('stops') as Stop[]
  if (!Array.isArray(stops) || !stops.length || !stopGroup.value) return

  for (let i = 0; i < stops.length; i++) {
    const {stop_lat, stop_lon, stop_name, stop_id} = stops[i]!

    const marker = L.marker([stop_lat, stop_lon], {icon: stopIcon})
    marker.bindTooltip(stop_name, {
      direction: 'top',
      offset: [0, -14],
      className: 'stop-name-tooltip',
    })
    marker.on('click', () => {
      router.push({name: 'stop', params: {stopId: stop_id}, replace: true})
    })

    marker.addTo(stopGroup.value)
    stopMarkers.set(stop_id.toString(), marker)
    stopNames.set(stop_id.toString(), stop_name)
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

const isInClujCounty = (lat: number, lng: number) => {
  return lat >= CLUJ_COUNTY_SW[0] && lat <= CLUJ_COUNTY_NE[0]
    && lng >= CLUJ_COUNTY_SW[1] && lng <= CLUJ_COUNTY_NE[1]
}

const updateLiveLocation = (e: L.LocationEvent) => {
  if (!map.value) return

  if (!isInClujCounty(e.latlng.lat, e.latlng.lng)) {
    userStore.setHasLocationPermission(false)
    userStore.clearUserLocation()
    if (userDot.value) {
      map.value.removeLayer(userDot.value)
      userDot.value = undefined
    }
    if (accuracyCircle.value) {
      map.value.removeLayer(accuracyCircle.value)
      accuracyCircle.value = undefined
    }
    return
  }

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
      interactive: false,
      zIndexOffset: 10000
    }).addTo(map.value)
  }
}

onMounted(() => {
  mapInit(46.7712, 23.6236, DEFAULT_ZOOM)
  void stopsInit()

  window.addEventListener('resize', scheduleInvalidateMapSize, {passive: true})
  window.addEventListener('orientationchange', scheduleInvalidateMapSize)
  window.visualViewport?.addEventListener('resize', scheduleInvalidateMapSize)

  scheduleInvalidateMapSize()
})

watch(() => route.params.stopId, (newId) => {
  highlightSelectedStop(newId as string)
})

watch(easterEggActive, (active) => {
  mapContainer.value?.classList.toggle('hungry-theme', active)
  if (currentlyHighlightedStopId.value && stopMarkers.has(currentlyHighlightedStopId.value)) {
    const marker = stopMarkers.get(currentlyHighlightedStopId.value)!
    marker.setIcon(makeSelectedStopIcon())
  }
})

watch(isDarkMode, (newValue) => {
  if (!currentTileLayer.value) return

  currentTileLayer.value.setUrl(getTileLayerUrl(newValue))
})

const updateSearchPlaceholder = () => {
  const searchInput = document.querySelector('.leaflet-control-geosearch input.glass') as HTMLInputElement | null
  if (searchInput) searchInput.placeholder = t('Search location...')
}

watch(locale, updateSearchPlaceholder)

const getShapeColorCounts = (shapes: ShapeLayerEntry[]) => {
  const colorCounts = new Map<string, number>()
  for (const [displayShape, shapeData] of shapes) {
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue
    const routeColor = displayShape.route_color
    colorCounts.set(routeColor, (colorCounts.get(routeColor) ?? 0) + 1)
  }
  return colorCounts
}

const getShapeSignature = (latLngs: L.LatLngTuple[], dashArray: string) => {
  return `${dashArray}|${latLngs.map(ll => `${ll[0].toFixed(4)},${ll[1].toFixed(4)}`).join('|')}`
}

const addGroupedStart = (
  groupedStarts: Map<string, GroupedStart>,
  startPoint: L.LatLngTuple,
  routeName: string,
  routeColor: string,
) => {
  const key = `${startPoint[0].toFixed(4)},${startPoint[1].toFixed(4)}`
  if (!groupedStarts.has(key)) groupedStarts.set(key, {lat: startPoint[0], lng: startPoint[1], routes: []})
  const existing = groupedStarts.get(key)!
  if (!existing.routes.some((r) => r.name === routeName)) {
    existing.routes.push({name: routeName, color: routeColor})
  }
}

const addRouteEndMarker = (layerGroup: L.FeatureGroup, endPoint: L.LatLngTuple, routeColor: string) => {
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
  L.marker(endPoint, {icon: endMarkerIcon}).addTo(layerGroup)
}

const addGroupedStartMarkers = (layerGroup: L.FeatureGroup, groupedStarts: Map<string, GroupedStart>) => {
  groupedStarts.forEach((data) => {
    const routesHtml = data.routes.map((r) =>
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
    L.marker([data.lat, data.lng], {icon: startMarkerIcon}).addTo(layerGroup)
  })
}

const renderShapes = (newShapes: ShapeLayerEntry[]) => {
  if (!shapeLayerGroup.value) return

  const layerGroup = shapeLayerGroup.value
  layerGroup.clearLayers()
  const drawnPaths = new Set<string>()
  const groupedStarts = new Map<string, GroupedStart>()
  const hasHighlights = highlightedStops.value.length > 0
  const colorCounts = getShapeColorCounts(newShapes)
  const colorSeen = new Map<string, number>()

  for (const [displayShape, shapeData] of newShapes) {
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue

    const latLngs: L.LatLngTuple[] = shapeData.map((sd) => [sd.shape_pt_lat, sd.shape_pt_lon])
    const routeColor = displayShape.route_color
    routeColorsCache.set(displayShape.trip_id, routeColor)

    const colorVariantIdx = colorSeen.get(routeColor) ?? 0
    colorSeen.set(routeColor, colorVariantIdx + 1)
    const dashArray = (colorCounts.get(routeColor) ?? 0) > 1
      ? (DUPLICATE_DASH_PATTERNS[colorVariantIdx % DUPLICATE_DASH_PATTERNS.length] ?? '')
      : ''

    const signature = getShapeSignature(latLngs, dashArray)
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
    }).addTo(layerGroup)

    if (hasHighlights) continue

    const startPoint = latLngs[0]
    if (startPoint && displayShape.route_short_name) {
      addGroupedStart(groupedStarts, startPoint, displayShape.route_short_name, routeColor)
    }

    const endPoint = latLngs[latLngs.length - 1]
    if (endPoint) addRouteEndMarker(layerGroup, endPoint, routeColor)
  }

  if (!hasHighlights) addGroupedStartMarkers(layerGroup, groupedStarts)
  if (zoomOut.value) zoomOut.value = false
}

watch(shapesToDisplay, (newShapes) => {
  renderShapes(newShapes as ShapeLayerEntry[])
}, {deep: true})

const BUS_STOP_PATH = 'M4 16c0 .88.39 1.67 1 2.22V20c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1h8v1c0 .55.45 1 1 1h1c.55 0 1-.45 1-1v-1.78c.61-.55 1-1.34 1-2.22V6c0-3.5-3.58-4-8-4s-8 .5-8 4v10zm3.5 1c-.83 0-1.5-.67-1.5-1.5S6.67 14 7.5 14s1.5.67 1.5 1.5S8.33 17 7.5 17zm9 0c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm-10-7V6h11v4H6.5z'

const makeHighlightIcon = (color: 'green' | 'purple' | 'red' | 'gray') => {
  const bg = color === 'green' ? '#10b981' : color === 'purple' ? '#a855f7' : color === 'red' ? '#f43f5e' : '#64748b'

  if (easterEggActive.value) {
    return L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `<div style="width:24px;height:30px;display:flex;align-items:flex-end;justify-content:center;">
        <svg viewBox="0 0 12 16" width="22" height="28" xmlns="http://www.w3.org/2000/svg">
          <path d="M1,15 L1,5.5 A5,5 0 0,1 11,5.5 L11,15 L9,12.5 L7,15 L5,12.5 L3,15 Z" fill="${bg}"/>
          <circle cx="3.8" cy="8" r="1.3" fill="white"/>
          <circle cx="7.8" cy="8" r="1.3" fill="white"/>
          <circle cx="4.2" cy="8.4" r="0.7" fill="rgba(0,0,0,0.65)"/>
          <circle cx="8.2" cy="8.4" r="0.7" fill="rgba(0,0,0,0.65)"/>
        </svg>
      </div>`,
      iconSize: [24, 30],
      iconAnchor: [12, 28],
    })
  }

  return L.divIcon({
    className: 'bg-transparent border-none !overflow-visible',
    html: `<div style="width:24px;height:24px;border-radius:50%;background:${bg};border:2px solid white;box-shadow:0 1px 4px rgba(0,0,0,0.22);display:flex;align-items:center;justify-content:center;">
      <svg viewBox="0 0 24 24" fill="white" width="14" height="14"><path d="${BUS_STOP_PATH}"/></svg>
    </div>`,
    iconSize: [24, 24],
    iconAnchor: [12, 12],
  })
}

watch([highlightedStops, currentlyHighlightedStopId, easterEggActive], ([stops]) => {
  if (!highlightedStopLayerGroup.value) return
  highlightedStopLayerGroup.value.clearLayers()
  const selectedId = currentlyHighlightedStopId.value
  for (const {stopId, color} of stops as HighlightedStop[]) {
    if (stopId === selectedId) continue
    const marker = stopMarkers.get(stopId)
    if (!marker) continue
    const latlng = marker.getLatLng()
    const name = stopNames.get(stopId)
    const m = L.marker(latlng, {
      icon: makeHighlightIcon(color),
      zIndexOffset: color === 'green' ? 1200 : color === 'purple' ? 1100 : color === 'red' ? 1000 : 800,
      interactive: true,
    })
    if (name) {
      m.bindTooltip(name, {
        direction: 'top',
        offset: [0, -14],
        className: 'stop-name-tooltip',
      })
    }
    m.on('click', () => router.push({name: 'stop', params: {stopId}}))
    m.addTo(highlightedStopLayerGroup.value!)
  }
}, {deep: true})

const getVehicleMarkerHtml = (
  vehicle: DisplayVehicle,
  resolvedColor: string,
  isStopView: boolean,
  showStopInfo: boolean,
) => {
  const routeName = vehicle.route_short_name || ''
  const routeFontSize = routeName.length >= 4 ? 8 : routeName.length >= 3 ? 9 : 11
  const roundedSpeed = Math.round(vehicle.speed)
  const titleText = routeName ? `${routeName} • ${vehicle.label}` : vehicle.label
  const heading = vehicle.heading || 0

  if (easterEggActive.value) {
    // Pac-Man faces right at 0°; heading 0 = north, so subtract 90° to align
    const rotation = heading - 90
    const pacman = `<div style="transform:rotate(${rotation}deg);flex-shrink:0;">
      <div class="pacman-eat" style="width:${isStopView ? 36 : 32}px;height:${isStopView ? 36 : 32}px;background-color:${resolvedColor};border-radius:50%;border:2px solid white;box-shadow:0 2px 8px rgba(0,0,0,0.28);"></div>
    </div>`

    if (isStopView) {
      return `
        <div style="position:relative;display:flex;flex-direction:column;align-items:center;gap:1px;">
          ${pacman}
          <div style="background-color:${resolvedColor};color:white;font-size:${routeFontSize}px;font-weight:900;padding:0 3px;border-radius:3px;border:1px solid rgba(255,255,255,0.8);line-height:1.5;white-space:nowrap;">${routeName}</div>
          ${showStopInfo ? `
            <div class="absolute" style="left:42px;top:0;background:rgba(15,23,42,0.9);color:#f1f5f9;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;">
              <span style="font-weight:700;font-size:14px;">${titleText}</span>
              <span style="font-size:12px;color:#94a3b8;">${roundedSpeed} km/h</span>
            </div>
          ` : ''}
        </div>`
    }

    return `
      <div style="position:relative;display:flex;align-items:center;">
        ${pacman}
        <div class="absolute" style="left:40px;background:rgba(15,23,42,0.9);color:#f1f5f9;padding:4px 10px;border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,0.3);display:flex;flex-direction:column;white-space:nowrap;z-index:20;pointer-events:none;">
          <span style="font-weight:700;font-size:14px;">${titleText}</span>
          <span style="font-size:12px;color:#94a3b8;">${roundedSpeed} km/h</span>
        </div>
      </div>`
  }

  return isStopView
    ? `
      <div class="relative flex items-center">
        <div class="flex items-center justify-center w-9 h-9 rounded-full border-2 border-white shadow-md z-30"
             style="background-color: ${resolvedColor};">
          <span class="font-black leading-none text-white tracking-tight"
                style="font-size:${routeFontSize}px;max-width:22px;">${routeName}</span>
        </div>
        <div class="absolute -right-0.5 -bottom-0.5 w-4 h-4 rounded-full border border-white bg-slate-900/85 flex items-center justify-center shadow-sm z-40">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white" class="w-2.5 h-2.5 shrink-0"
               style="transform: rotate(${heading}deg);">
            <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
          </svg>
        </div>
        ${showStopInfo ? `
          <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md flex flex-col whitespace-nowrap z-20 pointer-events-none">
            <span class="font-bold text-sm tracking-wide">${titleText}</span>
            <span class="text-xs text-slate-400">${roundedSpeed} km/h</span>
          </div>
        ` : ''}
      </div>
    `
    : `
      <div class="relative flex items-center">
        <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-md z-30 shrink-0"
             style="background-color: ${resolvedColor};">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white" class="w-4 h-4"
               style="transform: rotate(${heading}deg);">
            <path d="M12 2L21 21l-9-4-9 4 9-19z" stroke="currentColor" stroke-width="1" stroke-linejoin="round"/>
          </svg>
        </div>
        <div class="absolute left-10 bg-slate-900/90 dark:bg-slate-800/90 text-slate-100 px-2.5! py-1! rounded-md shadow-md flex flex-col whitespace-nowrap z-20 pointer-events-none">
          <span class="font-bold text-sm tracking-wide">${titleText}</span>
          <span class="text-xs text-slate-400">${roundedSpeed} km/h</span>
        </div>
      </div>
    `
}

const renderVehicles = (vehicles: DisplayVehicle[], currentRouteName: string | symbol | null | undefined) => {
  if (!vehicleLayerGroup.value) return

  const layerGroup = vehicleLayerGroup.value
  layerGroup.clearLayers()
  const isStopView = currentRouteName === 'stop'

  for (const vehicle of vehicles) {
    if (vehicle.latitude <= 0 || vehicle.longitude <= 0) continue

    const resolvedColor = routeColorsCache.get(vehicle.trip_id) || vehicle.route_color
    if (!resolvedColor) continue

    const showStopInfo = isStopView && selectedStopVehicleId.value === vehicle.id
    const markerHtml = getVehicleMarkerHtml(vehicle, resolvedColor, isStopView, showStopInfo)
    const marker = L.marker([vehicle.latitude, vehicle.longitude], {
      icon: L.divIcon({
        className: 'bg-transparent border-none !overflow-visible',
        html: markerHtml,
        iconSize: isStopView ? [36, 36] : [32, 32],
        iconAnchor: isStopView ? [18, 18] : [16, 16],
      }),
      zIndexOffset: showStopInfo ? 5600 : 5000
    })

    if (isStopView) {
      marker.on('click', () => {
        selectedStopVehicleId.value = selectedStopVehicleId.value === vehicle.id ? null : vehicle.id
      })
    }

    marker.addTo(layerGroup)
  }
}

watch([vehiclesToDisplay, () => route.name, selectedStopVehicleId, easterEggActive], ([vehicles, routeName]) => {
  renderVehicles(vehicles as DisplayVehicle[], routeName)
}, {deep: true})

watch(() => route.name, (name) => {
  if (name !== 'stop') selectedStopVehicleId.value = null
})

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
  window.removeEventListener('resize', scheduleInvalidateMapSize)
  window.removeEventListener('orientationchange', scheduleInvalidateMapSize)
  window.visualViewport?.removeEventListener('resize', scheduleInvalidateMapSize)

  if (resizeRaf) cancelAnimationFrame(resizeRaf)
  if (resizeTimer) clearTimeout(resizeTimer)

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
