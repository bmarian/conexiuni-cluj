<script setup lang="ts">
import {onMounted, onUnmounted, ref, shallowRef, watch} from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import {GeoSearchControl, OpenStreetMapProvider} from 'leaflet-geosearch'
import 'leaflet-geosearch/dist/geosearch.css'
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";
import {apiRequest} from "@/utils/request_cache.ts";
import type {Stop} from "@/types/tranzy.ts";
import {useRoute, useRouter} from "vue-router";
import {useI18n} from "vue-i18n";
import {type DisplayShape, type HighlightedStop, useMapStore} from "@/stores/map.ts";
import type {ShapePoint} from "@/types/map.ts";
import {useSettingsStore} from "@/stores/settings.ts";
import {useFavoritesStore} from "@/stores/favorites.ts";
import {
  type DisplayVehicle,
  type IconThemeOptions,
  makeStopIcon,
  makeSelectedStopIcon,
  makeHighlightIcon,
  getVehicleMarkerHtml,
} from '@/utils/mapIcons.ts'

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapStore = useMapStore()
const settingsStore = useSettingsStore()
const favoritesStore = useFavoritesStore()
const {shapesToDisplay, centerOnUser, zoomOut, vehiclesToDisplay, highlightedStops} = storeToRefs(mapStore)
const {easterEggActive, traditionalActive} = storeToRefs(settingsStore)
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

type ShapeLayerEntry = [DisplayShape, ShapePoint[]]
type GroupedStart = {
  lat: number
  lng: number
  routes: { name: string, color: string }[]
}

const getTileLayerUrl = (useDarkMode: boolean) => {
  return `https://{s}.basemaps.cartocdn.com/${useDarkMode ? 'dark_all' : 'light_all'}/{z}/{x}/{y}{r}.png`
}


const themeOpts = (): IconThemeOptions => ({
  easterEggActive: easterEggActive.value,
  traditionalActive: traditionalActive.value,
})

const stopIconForId = (stopId: string) =>
  makeStopIcon(favoritesStore.isStopFavorite(parseInt(stopId)), themeOpts())

const highlightSelectedStop = (stopId?: string) => {
  if (currentlyHighlightedStopId.value && stopMarkers.has(currentlyHighlightedStopId.value)) {
    const oldMarker = stopMarkers.get(currentlyHighlightedStopId.value)!
    oldMarker.setIcon(stopIconForId(currentlyHighlightedStopId.value))
    oldMarker.setZIndexOffset(0)
    if (map.value && map.value.hasLayer(oldMarker)) map.value.removeLayer(oldMarker)
    if (stopGroup.value && !stopGroup.value.hasLayer(oldMarker)) stopGroup.value.addLayer(oldMarker)
  }
  currentlyHighlightedStopId.value = null
  if (stopId && stopMarkers.has(stopId)) {
    const newMarker = stopMarkers.get(stopId)!
    newMarker.setIcon(makeSelectedStopIcon(themeOpts()))
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
  const control = new L.Control({position: 'topleft'})
  control.onAdd = () => {
    const container = L.DomUtil.create('div', 'leaflet-bar leaflet-control')

    const centerBtn = L.DomUtil.create('a', 'flex! items-center justify-center', container)
    centerBtn.href = '#'
    centerBtn.title = t('Center')
    centerBtn.setAttribute('role', 'button')
    centerBtn.innerHTML = `
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-[18px] h-[18px] text-slate-700 dark:text-slate-300">
        <circle cx="12" cy="12" r="4"/>
        <path d="M12 2v2"/>
        <path d="M12 20v2"/>
        <path d="M5 12H2"/>
        <path d="M22 12h-3"/>
      </svg>
    `
    L.DomEvent.on(centerBtn, 'click', (e) => {
      e.preventDefault()
      const location = userDot.value?.getLatLng()
      if (!location) return
      mapValue.flyTo(location, DEFAULT_ZOOM, {duration: 1})
    })

    const fitBtn = L.DomUtil.create('a', 'flex! items-center justify-center', container)
    fitBtn.href = '#'
    fitBtn.title = t('fitRoutes')
    fitBtn.setAttribute('role', 'button')
    fitBtn.innerHTML = `
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-[18px] h-[18px] text-slate-700 dark:text-slate-300">
        <path d="M8 3H5a2 2 0 0 0-2 2v3"/>
        <path d="M21 8V5a2 2 0 0 0-2-2h-3"/>
        <path d="M3 16v3a2 2 0 0 0 2 2h3"/>
        <path d="M16 21h3a2 2 0 0 0 2-2v-3"/>
      </svg>
    `
    L.DomEvent.on(fitBtn, 'click', (e) => {
      e.preventDefault()
      const bounds = shapeLayerGroup.value?.getBounds()
      if (!bounds || !bounds.isValid()) return
      mapValue.fitBounds(bounds, {padding: [24, 24], maxZoom: 16, animate: true, duration: 0.8})
    })

    L.DomEvent.disableClickPropagation(container)
    return container
  }

  return control
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
  mapContainer.value?.classList.toggle('traditional-theme', traditionalActive.value)
}

const stopsInit = async () => {
  const stops = await apiRequest('stops') as Stop[]
  if (!Array.isArray(stops) || !stops.length || !stopGroup.value) return

  for (let i = 0; i < stops.length; i++) {
    const {stop_lat, stop_lon, stop_name, stop_id} = stops[i]!

    const marker = L.marker([stop_lat, stop_lon], {icon: stopIconForId(stop_id.toString())})
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
  stopMarkers.forEach((marker, id) => {
    if (id === currentlyHighlightedStopId.value) return
    marker.setIcon(stopIconForId(id))
  })
  if (currentlyHighlightedStopId.value && stopMarkers.has(currentlyHighlightedStopId.value)) {
    const marker = stopMarkers.get(currentlyHighlightedStopId.value)!
    marker.setIcon(makeSelectedStopIcon(themeOpts()))
  }
})

watch(traditionalActive, (active) => {
  mapContainer.value?.classList.toggle('traditional-theme', active)
  stopMarkers.forEach((marker, id) => {
    if (id === currentlyHighlightedStopId.value) return
    marker.setIcon(stopIconForId(id))
  })
  if (currentlyHighlightedStopId.value && stopMarkers.has(currentlyHighlightedStopId.value)) {
    const marker = stopMarkers.get(currentlyHighlightedStopId.value)!
    marker.setIcon(makeSelectedStopIcon(themeOpts()))
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
      color: traditionalActive.value ? '#003C9C' : '#94a3b8',
      weight: traditionalActive.value ? 4 : 5,
      opacity: traditionalActive.value ? 0.85 : 0.7,
      dashArray: easterEggActive.value ? '0 14' : (dashArray || undefined),
      smoothFactor: 1.5,
      lineJoin: traditionalActive.value ? 'miter' : 'round',
      lineCap: traditionalActive.value ? 'butt' : 'round'
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

watch([shapesToDisplay, easterEggActive, traditionalActive], ([newShapes]) => {
  renderShapes(newShapes as ShapeLayerEntry[])
}, {deep: true})


watch([highlightedStops, currentlyHighlightedStopId, easterEggActive, traditionalActive], ([stops]) => {
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
      icon: makeHighlightIcon(color, themeOpts()),
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
    const markerHtml = getVehicleMarkerHtml(vehicle, resolvedColor, isStopView, showStopInfo, themeOpts())
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

watch([vehiclesToDisplay, () => route.name, selectedStopVehicleId, easterEggActive, traditionalActive], ([vehicles, routeName]) => {
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
