<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, shallowRef, watch, watchEffect} from 'vue'
import {
  type GeoJSONSource,
  type IControl,
  type LineLayerSpecification,
  LngLatBounds,
  Map as MapLibreMap,
  type Marker,
  setWorkerUrl,
  type StyleSpecification,
} from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import type {Feature, Polygon} from 'geojson'
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";
import {useStopsApi} from "@/composables/useStopsApi.ts";
import {useRoute, useRouter} from "vue-router";
import {useI18n} from "vue-i18n";
import {type DisplayShape, type HighlightedStop, useMapStore} from "@/stores/map.ts";
import type {ShapePoint} from "@/types/map.ts";
import {useSettingsStore} from "@/stores/settings.ts";
import {useFavoritesStore} from "@/stores/favorites.ts";
import {calculateBearing, haversineMeters} from "@/utils/geo.ts";
import {
  type DisplayVehicle,
  type MarkerIcon,
  getVehicleMarkerHtml,
  type IconThemeOptions,
  makeHighlightIcon,
  makePinIcon,
  makeSelectedStopIcon,
  makeStopIcon,
} from '@/utils/mapIcons.ts'
import {
  CLICK_TOLERANCE_PX,
  createTooltip,
  htmlMarker,
  type HtmlMarkerOptions,
  type LatLng,
  markerLatLng,
  onMarkerClick,
  setMarkerIcon,
} from '@/utils/mapMarkers.ts'
import {mapCreditsHTML, mapStyle, type MapTheme} from '@/utils/mapStyles.ts'
import {NAV_POSITION_OPTIONS} from '@/utils/journey.ts'
import {cacheKey, cities, cityBounds, cityCenter, currentCity, inCity, type City} from '@/utils/city.ts'

setWorkerUrl(workerUrl)

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapStore = useMapStore()
const settingsStore = useSettingsStore()
const favoritesStore = useFavoritesStore()
const {
  shapesToDisplay,
  walkingPolylines,
  centerOnUser,
  flyToLocation,
  pinnedLocation,
  customOriginLocation,
  zoomOut,
  vehiclesToDisplay,
  highlightedStops,
  directionArrowAtStart,
  drawerBottomPx,
  drawerRightPx,
  rideAs,
  preciseLocation,
} = storeToRefs(mapStore)
const {showVehicleExtras} = storeToRefs(settingsStore)
// The watch layout only has the default look.
const arcadeActive = computed(() => settingsStore.arcadeActive && !settingsStore.watchActive)
const legacyBlueActive = computed(() => settingsStore.legacyBlueActive && !settingsStore.watchActive)
const paperActive = computed(() => settingsStore.paperActive && !settingsStore.watchActive)
const router = useRouter()
const route = useRoute()
const stopData = new Map<string, {lat: number; lng: number; name: string}>()
const stopMarkers = new Map<string, Marker>()
const shownStops = new Set<string>()
const currentlyHighlightedStopId = ref<string | null>(null)
const selectedVehicleId = ref<number | null>(null)
const {t} = useI18n()
const mapContainer = ref()

const map = shallowRef<MapLibreMap>()
const userDot = shallowRef<Marker>()
const pinMarker = shallowRef<Marker>()
const originMarker = shallowRef<Marker>()
let shapeMarkers: Marker[] = []
let vehicleMarkers: Marker[] = []
let highlightMarkers: Marker[] = []
let tooltip: ReturnType<typeof createTooltip> | null = null
const routeColorsCache = new Map<string | number, string>()

let isFirstLocationHandle = true
let hasFittedForContent = false
let pendingCenterOnUser = false
let resizeRaf = 0
let resizeTimer: ReturnType<typeof setTimeout> | null = null
let locationRetryTimer: ReturnType<typeof setTimeout> | null = null
let geoWatchId: number | null = null
let stopRefreshFrame = 0
let styleReady = false

const DEFAULT_ZOOM = 15
const STOP_ZOOM_THRESHOLD = 15
const FIT_MAX_ZOOM = 15
const MIN_ZOOM = 8
const MAX_ZOOM = 19
const mapViewKey = (slug = currentCity.value) => cacheKey('map.lastView', slug)
const DUPLICATE_DASH_PATTERNS = ['', '8 7', '2 7', '10 4 2 4', '1 6']
const LOCATION_RETRY_DELAY_MS = 1200
const FADED_VEHICLE_OPACITY = 0.5
const FADED_VEHICLE_SCALE = 0.7
const FLY_DURATION_MS = 1000
const CITY_FLIGHT_MS = 2500
const FIT_DURATION_MS = 800
const EARTH_RADIUS_M = 6371008.8
const OVERLAY = 'cc-'
const SLOTS = ['shapes', 'walks', 'accuracy'] as const

type LatLngTuple = [number, number]
type ShapeLayerEntry = [DisplayShape, ShapePoint[]]
type GroupedStart = {
  lat: number
  lng: number
  routes: { name: string, color: string }[]
}

type SavedMapView = {
  lat: number
  lng: number
  glZoom?: number
  zoom?: number
}

type LineGroup = 'shapes' | 'walks'
type LineSpec = {
  coords: LatLngTuple[]
  color: string
  width: number
  opacity: number
  dash?: number[]
  square: boolean
}

type Fix = LatLng & { accuracy: number, heading: number | null }

const lines: Record<LineGroup, LineSpec[]> = {shapes: [], walks: []}
const drawnLines: Record<LineGroup, number> = {shapes: 0, walks: 0}
let accuracy: (LatLng & { radius: number }) | null = null

const themeOpts = (): IconThemeOptions => ({
  arcadeActive: arcadeActive.value,
  legacyBlueActive: legacyBlueActive.value,
  paperActive: paperActive.value,
  showVehicleExtras: showVehicleExtras.value,
})

const paperInk = () => isDarkMode.value ? '#efe6cc' : '#1f3a7a'

const mapTheme = (): MapTheme =>
  paperActive.value ? 'paper' : arcadeActive.value ? 'arcade' : legacyBlueActive.value ? 'legacyBlue' : 'default'

const currentStyle = () => mapStyle(isDarkMode.value, mapTheme())

// Under the basemap's labels.
const labelIndex = (layers: StyleSpecification['layers']) => {
  let ground = -1
  layers.forEach((layer, i) => {
    if (layer.type !== 'symbol' && !layer.id.startsWith('boundary') && !layer.id.startsWith(OVERLAY)) ground = i
  })
  const label = layers.findIndex((layer, i) => i > ground && layer.type === 'symbol')
  return label === -1 ? layers.length : label
}

const slotId = (slot: typeof SLOTS[number]) => `${OVERLAY}slot-${slot}`

const ensureSlots = (m: MapLibreMap) => {
  if (m.getLayer(slotId('accuracy'))) return
  const layers = m.getStyle().layers
  const before = layers[labelIndex(layers)]?.id
  const empty = `${OVERLAY}empty`
  if (!m.getSource(empty)) m.addSource(empty, {type: 'geojson', data: {type: 'FeatureCollection', features: []}})
  for (const slot of SLOTS) m.addLayer({id: slotId(slot), type: 'line', source: empty}, before)
}

// Dash arrays are in line widths.
const dashOf = (pattern: string, width: number) =>
  pattern ? pattern.split(' ').map((n) => Number(n) / width) : undefined

const toLngLat = ([lat, lng]: LatLngTuple): LatLngTuple => [lng, lat]

const syncLines = (group: LineGroup) => {
  const m = map.value
  if (!m || !styleReady) return
  for (let i = 0; i < drawnLines[group]; i++) {
    const id = `${OVERLAY}${group}-${i}`
    if (m.getLayer(id)) m.removeLayer(id)
    if (m.getSource(id)) m.removeSource(id)
  }
  lines[group].forEach((line, i) => {
    const id = `${OVERLAY}${group}-${i}`
    m.addSource(id, {
      type: 'geojson',
      data: {type: 'Feature', properties: {}, geometry: {type: 'LineString', coordinates: line.coords.map(toLngLat)}},
    })
    const layer: LineLayerSpecification = {
      id,
      type: 'line',
      source: id,
      layout: {'line-cap': line.square ? 'butt' : 'round', 'line-join': line.square ? 'miter' : 'round'},
      paint: {'line-color': line.color, 'line-width': line.width, 'line-opacity': line.opacity},
    }
    if (line.dash) layer.paint!['line-dasharray'] = line.dash
    m.addLayer(layer, slotId(group))
  })
  drawnLines[group] = lines[group].length
}

const accuracyRing = ({lat, lng}: LatLng, radius: number): Feature<Polygon> => {
  const dLat = (radius / EARTH_RADIUS_M) * 180 / Math.PI
  const dLng = dLat / Math.cos(lat * Math.PI / 180)
  const steps = 64
  const ring = Array.from({length: steps + 1}, (_, i) => {
    const a = (i / steps) * 2 * Math.PI
    return [lng + dLng * Math.cos(a), lat + dLat * Math.sin(a)]
  })
  return {type: 'Feature', properties: {}, geometry: {type: 'Polygon', coordinates: [ring]}}
}

const syncAccuracy = () => {
  const m = map.value
  if (!m || !styleReady) return
  const id = `${OVERLAY}accuracy`
  if (!accuracy) {
    for (const layer of [`${id}-fill`, `${id}-line`]) if (m.getLayer(layer)) m.removeLayer(layer)
    if (m.getSource(id)) m.removeSource(id)
    return
  }
  const data = accuracyRing(accuracy, accuracy.radius)
  const source = m.getSource<GeoJSONSource>(id)
  if (source) {
    source.setData(data)
  } else {
    m.addSource(id, {type: 'geojson', data})
    m.addLayer({id: `${id}-fill`, type: 'fill', source: id, paint: {'fill-color': '#3b82f6'}}, slotId('accuracy'))
    m.addLayer({id: `${id}-line`, type: 'line', source: id, paint: {'line-color': '#3b82f6', 'line-width': 1}}, slotId('accuracy'))
  }
  m.setPaintProperty(`${id}-fill`, 'fill-opacity', rideAs.value ? 0 : 0.15)
  m.setPaintProperty(`${id}-line`, 'line-opacity', rideAs.value ? 0 : 1)
}

const syncOverlays = () => {
  const m = map.value
  if (!m || !styleReady) return
  ensureSlots(m)
  syncLines('shapes')
  syncLines('walks')
  syncAccuracy()
}

const applyStyle = () => {
  const m = map.value
  if (!m) return
  // A diff drops our layers; when the style is rebuilt instead, style.load brings them back.
  m.setStyle(currentStyle(), {diff: true})
  syncOverlays()
}

const cityMaxBounds = (): LngLatBounds | null => {
  const b = cityBounds()
  return b ? new LngLatBounds([b.west, b.south], [b.east, b.north]) : null
}

const linesBounds = (...groups: LineSpec[][]) => {
  const bounds = new LngLatBounds()
  for (const group of groups) for (const line of group) for (const point of line.coords) bounds.extend(toLngLat(point))
  return bounds.isEmpty() ? null : bounds
}

const fitPadding = () => ({
  top: 24,
  left: 24,
  right: 24 + drawerRightPx.value,
  bottom: 24 + drawerBottomPx.value,
})

const fitTo = (bounds: LngLatBounds) => {
  map.value?.fitBounds(bounds, {padding: fitPadding(), maxZoom: FIT_MAX_ZOOM, duration: FIT_DURATION_MS})
}

const clearMarkers = (markers: Marker[]) => {
  for (const marker of markers) {
    tooltip?.forget(marker)
    marker.remove()
  }
}

const useTouchDragLift = () =>
  typeof window !== 'undefined'
  && (window.matchMedia?.('(pointer: coarse)').matches || 'ontouchstart' in window)

const attachPlannerDragLift = (marker: Marker) => {
  if (!useTouchDragLift()) return
  marker.on('dragstart', () => marker.getElement().classList.add('planner-drag-active'))
  marker.on('dragend', () => marker.getElement().classList.remove('planner-drag-active'))
}

const stopIconForId = (stopId: string) =>
  makeStopIcon(favoritesStore.isStopFavorite(parseInt(stopId)), themeOpts())

// Only the stops around the view are on the map, so panning moves a few dozen markers, not all of them.
const refreshStops = () => {
  const m = map.value
  if (!m) return
  const wanted = new Set<string>()
  if (m.getZoom() >= STOP_ZOOM_THRESHOLD) {
    const bounds = m.getBounds()
    const padLat = (bounds.getNorth() - bounds.getSouth()) / 2
    const padLng = (bounds.getEast() - bounds.getWest()) / 2
    for (const [id, {lat, lng}] of stopData) {
      if (lat >= bounds.getSouth() - padLat && lat <= bounds.getNorth() + padLat
        && lng >= bounds.getWest() - padLng && lng <= bounds.getEast() + padLng) wanted.add(id)
    }
  }
  const selected = currentlyHighlightedStopId.value
  if (selected && stopData.has(selected)) wanted.add(selected)
  for (const id of shownStops) {
    if (wanted.has(id)) continue
    const marker = stopMarkers.get(id)
    if (marker) {
      tooltip?.forget(marker)
      marker.remove()
    }
    shownStops.delete(id)
  }
  for (const id of wanted) {
    if (shownStops.has(id)) continue
    stopMarker(id)?.addTo(m)
    shownStops.add(id)
  }
}

const scheduleStopRefresh = () => {
  if (stopRefreshFrame) return
  stopRefreshFrame = requestAnimationFrame(() => {
    stopRefreshFrame = 0
    refreshStops()
  })
}

const highlightSelectedStop = (stopId?: string) => {
  const previous = currentlyHighlightedStopId.value
  if (previous && stopMarkers.has(previous)) {
    const oldMarker = stopMarkers.get(previous)!
    setMarkerIcon(oldMarker, stopIconForId(previous))
    oldMarker.getElement().style.zIndex = ''
  }
  currentlyHighlightedStopId.value = null
  const newMarker = stopId ? stopMarker(stopId) : null
  if (stopId && newMarker) {
    setMarkerIcon(newMarker, makeSelectedStopIcon(themeOpts()))
    newMarker.getElement().style.zIndex = '1000'
    currentlyHighlightedStopId.value = stopId
  }
  refreshStops()
}

const invalidateMapSize = () => {
  map.value?.resize()
}

// Offset so flyTo centres the point in the currently visible map area.
const flyToVisible = ({lat, lng}: LatLng, zoom: number) => {
  map.value?.flyTo({
    center: [lng, lat],
    zoom,
    offset: [-drawerRightPx.value / 2, -drawerBottomPx.value / 2],
    duration: FLY_DURATION_MS,
  })
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

const isStandaloneApp = () =>
  window.matchMedia('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true

const fixOf = ({coords}: GeolocationPosition): Fix => ({
  lat: coords.latitude,
  lng: coords.longitude,
  accuracy: coords.accuracy,
  heading: coords.heading,
})

const stopLocate = () => {
  if (geoWatchId !== null) navigator.geolocation?.clearWatch(geoWatchId)
  geoWatchId = null
}

const onLocationError = (error: GeolocationPositionError) => {
  console.warn("GPS Error:", error.message)
  userStore.setIsLocating(false)
  pendingCenterOnUser = false
  if (error.code === error.PERMISSION_DENIED) {
    userStore.setHasLocationPermission(false)
  }
}

const beginLocationWatch = (enableHighAccuracy = false) => {
  // The watch has no usable location.
  if (!map.value || settingsStore.watchActive) return
  if (!navigator.geolocation) {
    userStore.setIsLocating(false)
    return
  }

  userStore.setIsLocating(true)
  stopLocate()
  geoWatchId = navigator.geolocation.watchPosition(
    (position) => updateLiveLocation(fixOf(position)),
    onLocationError,
    preciseLocation.value ? NAV_POSITION_OPTIONS : {
      enableHighAccuracy,
      maximumAge: 30000,
      timeout: enableHighAccuracy ? 15000 : 6000,
    },
  )
}

const requestCurrentLocation = (enableHighAccuracy = false) => {
  beginLocationWatch(enableHighAccuracy)

  if (!navigator.geolocation || settingsStore.watchActive) return
  navigator.geolocation.getCurrentPosition(
    (position) => {
      if (!map.value) return
      updateLiveLocation(fixOf(position))
    },
    (error) => {
      userStore.setIsLocating(false)
      pendingCenterOnUser = false
      if (error.code === error.PERMISSION_DENIED) {
        userStore.setHasLocationPermission(false)
        userStore.clearUserLocation()
      }
    },
    {
      enableHighAccuracy,
      maximumAge: 30000,
      timeout: enableHighAccuracy ? 15000 : 6000,
    },
  )
}

const retryLocationForStandaloneApp = () => {
  if (!isStandaloneApp() || document.visibilityState !== 'visible' || userStore.userLocation) return
  requestCurrentLocation(false)
}

const CENTER_SVG = `
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-[18px] h-[18px] text-slate-700 dark:text-slate-300">
    <circle cx="12" cy="12" r="4"/>
    <path d="M12 2v2"/>
    <path d="M12 20v2"/>
    <path d="M5 12H2"/>
    <path d="M22 12h-3"/>
  </svg>
`

const FIT_SVG = `
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-[18px] h-[18px] text-slate-700 dark:text-slate-300">
    <path d="M8 3H5a2 2 0 0 0-2 2v3"/>
    <path d="M21 8V5a2 2 0 0 0-2-2h-3"/>
    <path d="M3 16v3a2 2 0 0 0 2 2h3"/>
    <path d="M16 21h3a2 2 0 0 0 2-2v-3"/>
  </svg>
`

const barButton = (bar: HTMLElement, title: string, html: string, className: string, onClick: () => void) => {
  const button = document.createElement('a')
  button.className = className
  button.href = '#'
  button.title = title
  button.setAttribute('role', 'button')
  button.setAttribute('aria-label', title)
  button.innerHTML = html
  button.addEventListener('click', (e) => {
    e.preventDefault()
    onClick()
  })
  bar.append(button)
  return button
}

const barControl = (className: string, build: (bar: HTMLElement) => void): IControl => {
  const bar = document.createElement('div')
  bar.className = `maplibregl-ctrl map-bar ${className}`
  build(bar)
  return {onAdd: () => bar, onRemove: () => bar.remove()}
}

const createZoomControl = (mapValue: MapLibreMap) => barControl('map-zoom', (bar) => {
  const inButton = barButton(bar, 'Zoom in', '<span aria-hidden="true">+</span>', 'map-zoom-in', () => mapValue.zoomIn())
  const outButton = barButton(bar, 'Zoom out', '<span aria-hidden="true">&#x2212;</span>', 'map-zoom-out', () => mapValue.zoomOut())
  const update = () => {
    inButton.classList.toggle('is-disabled', mapValue.getZoom() >= mapValue.getMaxZoom())
    outButton.classList.toggle('is-disabled', mapValue.getZoom() <= mapValue.getMinZoom())
  }
  mapValue.on('zoom', update)
  update()
})

const createCenterControl = () => barControl('', (bar) => {
  barButton(bar, t('Center'), CENTER_SVG, 'flex! items-center justify-center', () => {
    const location = userDot.value ? markerLatLng(userDot.value) : null
    if (!location) {
      pendingCenterOnUser = true
      requestCurrentLocation(true)
      return
    }
    flyToVisible(location, DEFAULT_ZOOM)
  })
  barButton(bar, t('fitRoutes'), FIT_SVG, 'flex! items-center justify-center', () => {
    const bounds = linesBounds(lines.shapes)
    if (bounds) fitTo(bounds)
  })
})

const credits = document.createElement('div')
credits.className = 'maplibregl-ctrl maplibregl-ctrl-attrib'

watchEffect(() => { credits.innerHTML = mapCreditsHTML() })

const createCreditsControl = (): IControl => ({onAdd: () => credits, onRemove: () => credits.remove()})

const mapInit = (center: LatLng, zoom: number) => {
  const mapValue = new MapLibreMap({
    container: mapContainer.value,
    style: currentStyle(),
    center: [center.lng, center.lat],
    zoom,
    minZoom: MIN_ZOOM,
    maxZoom: MAX_ZOOM,
    maxBounds: cityMaxBounds() ?? undefined,
    attributionControl: false,
    clickTolerance: CLICK_TOLERANCE_PX,
    dragRotate: false,
    pitchWithRotate: false,
    touchPitch: false,
  })
  mapValue.touchZoomRotate.disableRotation()
  mapValue.keyboard.disableRotation()
  mapValue.getCanvas().tabIndex = -1

  map.value = mapValue
  mapValue.on('styledataloading', () => {
    styleReady = false
  })
  mapValue.on('style.load', () => {
    styleReady = true
    syncOverlays()
  })
  mapValue.on('move', scheduleStopRefresh)
  mapValue.on('moveend', () => {
    refreshStops()
    const mapCenter = mapValue.getCenter()
    // A cut-off flight between cities can stop over another city.
    if (!inCity(mapCenter.lat, mapCenter.lng)) return
    const view: SavedMapView = {
      lat: Number(mapCenter.lat.toFixed(6)),
      lng: Number(mapCenter.lng.toFixed(6)),
      glZoom: Number(mapValue.getZoom().toFixed(2)),
    }
    localStorage.setItem(mapViewKey(), JSON.stringify(view))
  })
  mapValue.on('click', (e) => {
    if ((e.originalEvent.target as Element | null)?.closest?.('.maplibregl-marker')) return
    selectedVehicleId.value = null
  })

  // Paper's grain sits over the map and under the markers.
  const grain = document.createElement('div')
  grain.className = 'map-grain'
  mapValue.getCanvas().after(grain)

  tooltip = createTooltip(mapValue)
  mapValue.addControl(createZoomControl(mapValue), 'top-left')
  mapValue.addControl(createCenterControl(), 'top-left')
  mapValue.addControl(createCreditsControl(), 'bottom-right')
  beginLocationWatch()

  mapContainer.value?.classList.toggle('arcade-theme', arcadeActive.value)
  mapContainer.value?.classList.toggle('legacy-blue-theme', legacyBlueActive.value)
}

const stopMarker = (id: string): Marker | undefined => {
  const existing = stopMarkers.get(id)
  if (existing) return existing
  const stop = stopData.get(id)
  if (!stop) return undefined
  const marker = htmlMarker(stop, stopIconForId(id))
  tooltip?.bind(marker, stop.name, 'stop-name-tooltip', 14)
  onMarkerClick(marker, () => {
    router.push({name: 'stop', params: {stopId: id}})
  })
  stopMarkers.set(id, marker)
  return marker
}

const stopsInit = async () => {
  const {stops: stopsRef, fetchStops} = useStopsApi()
  await fetchStops()
  const stops = stopsRef.value
  if (!stops.length || !map.value) return

  for (const {stop_lat, stop_lon, stop_name, stop_id} of stops) {
    stopData.set(stop_id.toString(), {lat: stop_lat, lng: stop_lon, name: stop_name})
  }
  highlightSelectedStop(route.params.stopId as string)
  // Highlights set before the markers existed were skipped.
  renderHighlightedStops()
}

const blueDotIcon: MarkerIcon = {
  html: `
    <div class="relative flex items-center justify-center w-4 h-4">
      <div class="absolute w-4 h-4 bg-blue-500 border-2 border-white rounded-full shadow-md z-10"></div>
      <div class="absolute w-full h-full bg-blue-400 rounded-full animate-ping opacity-75"></div>
    </div>
  `,
  size: [16, 16],
  anchor: [8, 8]
}

// On board the traveller is drawn as their bus, gliding between fixes.
let rideHeading = 0
let headingFrom: LatLng | null = null
let lastFixMs = 0
let glideFrame = 0

const userIcon = (): MarkerIcon => {
  const ride = rideAs.value
  if (!ride) return blueDotIcon
  const bus = {route_short_name: ride.line, heading: rideHeading, speed: 0, label: ''} as DisplayVehicle
  return {
    html: getVehicleMarkerHtml(bus, ride.color, false, {...themeOpts(), showVehicleExtras: false}, t),
    size: [36, 36],
    anchor: [18, 18],
  }
}

const moveUserDot = (to: LatLng, ms: number) => {
  const dot = userDot.value
  if (!dot) return
  cancelAnimationFrame(glideFrame)
  if (!rideAs.value || ms <= 0) {
    dot.setLngLat([to.lng, to.lat])
    return
  }
  const from = markerLatLng(dot)
  const start = performance.now()
  const step = (now: number) => {
    const k = Math.min(1, (now - start) / ms)
    dot.setLngLat([from.lng + (to.lng - from.lng) * k, from.lat + (to.lat - from.lat) * k])
    if (k < 1) glideFrame = requestAnimationFrame(step)
  }
  glideFrame = requestAnimationFrame(step)
}

const updateLiveLocation = (fix: Fix) => {
  const m = map.value
  if (!m) return

  userStore.setIsLocating(false)
  if (!inCity(fix.lat, fix.lng)) {
    pendingCenterOnUser = false
    userStore.setHasLocationPermission(false)
    userStore.clearUserLocation()
    userDot.value?.remove()
    userDot.value = undefined
    accuracy = null
    syncAccuracy()
    return
  }

  userStore.setHasLocationPermission(true)
  userStore.setUserLocation(fix.lat, fix.lng)
  if (isFirstLocationHandle) {
    if (settingsStore.autoCenterOnMe && !hasFittedForContent) flyToVisible(fix, DEFAULT_ZOOM)
    isFirstLocationHandle = false
  }
  if (pendingCenterOnUser) {
    pendingCenterOnUser = false
    flyToVisible(fix, DEFAULT_ZOOM)
  }

  const now = Date.now()
  const sinceLast = lastFixMs ? now - lastFixMs : 0
  lastFixMs = now
  // Without a GPS heading, the direction from where the last one was taken, once beyond the fix's accuracy.
  if (fix.heading !== null && Number.isFinite(fix.heading)) {
    rideHeading = fix.heading
  } else if (!headingFrom) {
    headingFrom = {lat: fix.lat, lng: fix.lng}
  } else if (haversineMeters(headingFrom.lat, headingFrom.lng, fix.lat, fix.lng) > fix.accuracy) {
    rideHeading = calculateBearing(headingFrom.lat, headingFrom.lng, fix.lat, fix.lng)
    headingFrom = {lat: fix.lat, lng: fix.lng}
  }

  accuracy = {lat: fix.lat, lng: fix.lng, radius: fix.accuracy}
  syncAccuracy()

  if (userDot.value) {
    if (rideAs.value) setMarkerIcon(userDot.value, userIcon())
    moveUserDot(fix, sinceLast)
  } else {
    userDot.value = htmlMarker(fix, userIcon(), {interactive: false, zIndex: 10000}).addTo(m)
  }
}

const savedView = (slug: string): LatLng & { zoom: number } | null => {
  try {
    const parsed = JSON.parse(localStorage.getItem(mapViewKey(slug)) ?? 'null') as Partial<SavedMapView> | null
    if (!parsed) return null
    // Views saved by the Leaflet map are one zoom higher.
    const zoom = Number.isFinite(parsed.glZoom) ? parsed.glZoom as number : (parsed.zoom ?? NaN) - 1
    if (
      !Number.isFinite(parsed.lat)
      || !Number.isFinite(parsed.lng)
      || !Number.isFinite(zoom)
      || !inCity(parsed.lat as number, parsed.lng as number, slug)
    ) return null
    return {lat: parsed.lat as number, lng: parsed.lng as number, zoom: Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, zoom))}
  } catch {
    return null
  }
}

const openingView = (city: Pick<City, 'slug' | 'lat' | 'lon'>) =>
  savedView(city.slug) ?? {lat: city.lat, lng: city.lon, zoom: DEFAULT_ZOOM}

const clearStops = () => {
  for (const id of shownStops) {
    const marker = stopMarkers.get(id)
    if (!marker) continue
    tooltip?.forget(marker)
    marker.remove()
  }
  shownStops.clear()
  stopMarkers.clear()
  stopData.clear()
  routeColorsCache.clear()
}

let cityFlight = 0
let flyingBetweenCities = false

const clampToCity = () => {
  if (!flyingBetweenCities) map.value?.setMaxBounds(cityMaxBounds())
}

watch(cities, clampToCity)

watch(currentCity, (slug) => {
  const m = map.value
  const city = cities.value.find((c) => c.slug === slug)
  if (!m || !city) return
  clearStops()
  const view = openingView(city)
  const flight = ++cityFlight
  m.stop()
  flyingBetweenCities = true
  m.setMaxBounds(null)
  m.once('moveend', () => {
    if (flight !== cityFlight) return
    flyingBetweenCities = false
    clampToCity()
  })
  m.flyTo({center: [view.lng, view.lat], zoom: view.zoom, duration: CITY_FLIGHT_MS})
  void stopsInit()
})

onMounted(() => {
  const initialView = savedView(currentCity.value)
  if (initialView) {
    mapInit(initialView, initialView.zoom)
  } else {
    mapInit({lat: cityCenter().lat, lng: cityCenter().lon}, DEFAULT_ZOOM)
  }
  void stopsInit()
  // The watch mounts the map on demand, after a view has already filled the store.
  if (shapesToDisplay.value.length) renderShapes(shapesToDisplay.value as ShapeLayerEntry[])
  renderVehicles(vehiclesToDisplay.value as DisplayVehicle[])

  window.addEventListener('resize', scheduleInvalidateMapSize, {passive: true})
  window.addEventListener('orientationchange', scheduleInvalidateMapSize)
  window.addEventListener('pageshow', retryLocationForStandaloneApp)
  document.addEventListener('visibilitychange', retryLocationForStandaloneApp)
  window.visualViewport?.addEventListener('resize', scheduleInvalidateMapSize)

  scheduleInvalidateMapSize()
  locationRetryTimer = setTimeout(retryLocationForStandaloneApp, LOCATION_RETRY_DELAY_MS)
})

watch(() => route.params.stopId, (newId) => {
  highlightSelectedStop(newId as string)
})

const refreshStopIcons = () => {
  stopMarkers.forEach((marker, id) => {
    if (id === currentlyHighlightedStopId.value) return
    setMarkerIcon(marker, stopIconForId(id))
  })
  if (currentlyHighlightedStopId.value && stopMarkers.has(currentlyHighlightedStopId.value)) {
    const marker = stopMarkers.get(currentlyHighlightedStopId.value)!
    setMarkerIcon(marker, makeSelectedStopIcon(themeOpts()))
  }
}

watch(arcadeActive, (active) => {
  mapContainer.value?.classList.toggle('arcade-theme', active)
  refreshStopIcons()
})

watch(legacyBlueActive, (active) => {
  mapContainer.value?.classList.toggle('legacy-blue-theme', active)
  refreshStopIcons()
})

watch(paperActive, refreshStopIcons)

watch([isDarkMode, paperActive, arcadeActive, legacyBlueActive], applyStyle)

const getShapeColorCounts = (shapes: ShapeLayerEntry[]) => {
  const colorCounts = new Map<string, number>()
  for (const [displayShape, shapeData] of shapes) {
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue
    const routeColor = displayShape.route_color
    colorCounts.set(routeColor, (colorCounts.get(routeColor) ?? 0) + 1)
  }
  return colorCounts
}

const getShapeSignature = (latLngs: LatLngTuple[], dashArray: string) => {
  return `${dashArray}|${latLngs.map(ll => `${ll[0].toFixed(4)},${ll[1].toFixed(4)}`).join('|')}`
}

const addGroupedStart = (
  groupedStarts: Map<string, GroupedStart>,
  startPoint: LatLngTuple,
  routeName: string,
  routeColor: string,
) => {
  const key = `${startPoint[0].toFixed(4)},${startPoint[1].toFixed(4)}`
  if (!groupedStarts.has(key)) groupedStarts.set(key, {
    lat: startPoint[0],
    lng: startPoint[1],
    routes: []
  })
  const existing = groupedStarts.get(key)!
  if (!existing.routes.some((r) => r.name === routeName)) {
    existing.routes.push({name: routeName, color: routeColor})
  }
}

const computeStartBearing = (latLngs: LatLngTuple[]): number | null => {
  if (latLngs.length < 2) return null
  const [startLat, startLng] = latLngs[0]!
  for (let i = 1; i < Math.min(latLngs.length, 20); i++) {
    const [lat, lng] = latLngs[i]!
    if (haversineMeters(startLat, startLng, lat, lng) >= 30) {
      return calculateBearing(startLat, startLng, lat, lng)
    }
  }
  const [nextLat, nextLng] = latLngs[1]!
  return calculateBearing(startLat, startLng, nextLat, nextLng)
}

const addShapeMarker = (m: MapLibreMap, [lat, lng]: LatLngTuple, icon: MarkerIcon, options: HtmlMarkerOptions = {}) => {
  shapeMarkers.push(htmlMarker({lat, lng}, icon, options).addTo(m))
}

const addDirectionArrowMarker = (
  m: MapLibreMap,
  startPoint: LatLngTuple,
  bearing: number,
) => {
  const isLegacyBlue = legacyBlueActive.value
  const isArcade = arcadeActive.value
  const arrowSvg = `<svg viewBox="0 0 24 24" width="12" height="12" xmlns="http://www.w3.org/2000/svg" style="transform: rotate(${bearing}deg); display:block;">
    <path d="M12 3 L20 19 L12 15 L4 19 Z" fill="white" stroke="white" stroke-width="1" stroke-linejoin="round"/>
  </svg>`

  let html: string
  if (paperActive.value) {
    html = `<div style="display:flex;flex-direction:column;align-items:center;pointer-events:none;">
      <div style="width:22px;height:22px;background:var(--pp-ink);color:var(--pp-paper);display:flex;align-items:center;justify-content:center;">
        <svg viewBox="0 0 24 24" width="12" height="12" xmlns="http://www.w3.org/2000/svg" fill="currentColor" style="transform: rotate(${bearing}deg); display:block;">
          <path d="M12 3 L20 19 L12 15 L4 19 Z"/>
        </svg>
      </div>
      <div style="width:2px;height:8px;background:var(--pp-ink);"></div>
    </div>`
  } else if (isLegacyBlue) {
    html = `<div style="display:flex;flex-direction:column;align-items:center;pointer-events:none;">
      <div style="width:22px;height:22px;background:#000000;border:2px solid #FFFFFF;box-shadow:1px 1px 0 rgba(0,0,0,0.5);display:flex;align-items:center;justify-content:center;">
        ${arrowSvg}
      </div>
      <div style="width:0;height:0;border-left:5px solid transparent;border-right:5px solid transparent;border-top:6px solid #000000;margin-top:-1px;"></div>
    </div>`
  } else if (isArcade) {
    html = `<div style="display:flex;flex-direction:column;align-items:center;pointer-events:none;filter:drop-shadow(0 2px 4px rgba(0,0,0,0.35));">
      <div style="width:22px;height:22px;border-radius:50%;background:#0f172a;border:2px solid #FACC15;display:flex;align-items:center;justify-content:center;">
        ${arrowSvg}
      </div>
      <div style="width:0;height:0;border-left:5px solid transparent;border-right:5px solid transparent;border-top:6px solid #0f172a;margin-top:-1px;"></div>
    </div>`
  } else {
    const arrowSvgThemed = `<svg viewBox="0 0 24 24" width="12" height="12" xmlns="http://www.w3.org/2000/svg" fill="currentColor" style="transform: rotate(${bearing}deg); display:block;">
      <path d="M12 3 L20 19 L12 15 L4 19 Z"/>
    </svg>`
    html = `<div class="flex flex-col items-center pointer-events-none" style="filter:drop-shadow(0 2px 4px rgba(0,0,0,0.25));">
      <div class="flex items-center justify-center w-[22px] h-[22px] rounded-full border-2 border-white dark:border-[#0f172a] bg-slate-900 dark:bg-slate-100 text-white dark:text-slate-900">
        ${arrowSvgThemed}
      </div>
      <div class="dir-arrow-tail"></div>
    </div>`
  }

  addShapeMarker(m, startPoint, {
    html,
    size: [22, 32],
    anchor: [11, 44],
  }, {zIndex: 1500, interactive: false})
}

const addRouteEndMarker = (m: MapLibreMap, endPoint: LatLngTuple, routeColor: string) => {
  const isLegacyBlue = legacyBlueActive.value
  addShapeMarker(m, endPoint, {
    html: paperActive.value
      ? `<div style="width:18px;height:18px;background-color:${routeColor};border:1.5px solid var(--pp-ink);display:flex;align-items:center;justify-content:center;">
           <div style="width:6px;height:6px;background:var(--pp-paper);"></div>
         </div>`
      : isLegacyBlue
      ? `<div style="width:20px;height:20px;background-color:${routeColor};border:2px solid black;box-shadow:1px 1px 0 rgba(0,0,0,0.4);display:flex;align-items:center;justify-content:center;">
           <div style="width:6px;height:6px;background:white;border:1px solid rgba(0,0,0,0.4);"></div>
         </div>`
      : `<div class="flex items-center justify-center w-6 h-6 rounded-full border-[3px] border-white dark:border-[#0f172a] shadow-md z-20"
              style="background-color: ${routeColor};">
           <div class="w-1.5 h-1.5 bg-white dark:bg-[#0f172a] rounded-[2px]"></div>
         </div>`,
    size: [24, 24], anchor: [12, 12]
  })
}

const addGroupedStartMarkers = (m: MapLibreMap, groupedStarts: Map<string, GroupedStart>) => {
  const isLegacyBlue = legacyBlueActive.value
  groupedStarts.forEach((data) => {
    const routesHtml = data.routes.map((r) =>
      paperActive.value
        ? `<div class="pp-map-stamp" style="background-color:${r.color};font-size:13px;padding:3px 8px;line-height:1.3;white-space:nowrap;">${r.name}</div>`
        : isLegacyBlue
        ? `<div style="background-color:${r.color};color:white;font-size:10px;font-weight:700;padding:2px 6px;border:1px solid black;box-shadow:1px 1px 0 rgba(0,0,0,0.35);line-height:1.4;white-space:nowrap;font-family:'Tahoma','Trebuchet MS',sans-serif;">${r.name}</div>`
        : `<div style="background-color: ${r.color};"
                class="flex items-center justify-center min-w-[28px] h-[28px] px-2 rounded-full text-white text-[11px] font-black shadow-md border-[3px] border-white dark:border-[#0f172a]">
             ${r.name}
           </div>`
    ).join('')
    addShapeMarker(m, [data.lat, data.lng], {
      html: `<div class="absolute flex flex-row items-center justify-center gap-0.5 whitespace-nowrap" style="transform: translate(-50%, -50%);">${routesHtml}</div>`,
      size: [0, 0], anchor: [0, 0]
    })
  })
}

const renderShapes = (newShapes: ShapeLayerEntry[]) => {
  const m = map.value
  if (!m) return

  clearMarkers(shapeMarkers)
  shapeMarkers = []
  const shapeLines: LineSpec[] = []
  const drawnPaths = new Set<string>()
  const groupedStarts = new Map<string, GroupedStart>()
  const hasHighlights = highlightedStops.value.length > 0
  const colorCounts = getShapeColorCounts(newShapes)
  const colorSeen = new Map<string, number>()

  for (const [displayShape, shapeData] of newShapes) {
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue

    const latLngs: LatLngTuple[] = shapeData.map((sd) => [sd.shape_pt_lat, sd.shape_pt_lon])
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

    const square = legacyBlueActive.value || paperActive.value
    const width = square ? 4 : 5
    shapeLines.push({
      coords: latLngs,
      color: legacyBlueActive.value ? (isDarkMode.value ? '#4e88d8' : '#003C9C') : (routeColor || '#94a3b8'),
      width,
      opacity: paperActive.value ? 0.9 : 0.85,
      dash: dashOf(arcadeActive.value ? '0 14' : dashArray, width),
      square,
    })

    if (directionArrowAtStart.value && latLngs.length >= 2) {
      const bearing = computeStartBearing(latLngs)
      if (bearing !== null) {
        addDirectionArrowMarker(m, latLngs[0]!, bearing)
      }
    }

    if (hasHighlights) continue

    const startPoint = latLngs[0]
    if (startPoint && displayShape.route_short_name) {
      addGroupedStart(groupedStarts, startPoint, displayShape.route_short_name, routeColor)
    }

    const endPoint = latLngs[latLngs.length - 1]
    if (endPoint) addRouteEndMarker(m, endPoint, routeColor)
  }

  lines.shapes = shapeLines
  syncLines('shapes')
  if (!hasHighlights) addGroupedStartMarkers(m, groupedStarts)
  if (zoomOut.value) zoomOut.value = false
}

watch([shapesToDisplay, arcadeActive, legacyBlueActive, paperActive, directionArrowAtStart, isDarkMode], ([newShapes]) => {
  renderShapes(newShapes as ShapeLayerEntry[])
}, {deep: true})

watch(shapesToDisplay, (newShapes) => {
  // The watch frames its own view.
  if (!settingsStore.autoFitMap || !map.value || settingsStore.watchActive) return
  if (newShapes.length && mapStore.fitWalkingPolylines) return
  if (newShapes.length) {
    const bounds = linesBounds(lines.shapes)
    if (bounds) {
      fitTo(bounds)
      hasFittedForContent = true
    }
  }
}, {deep: true})

const renderWalkingPolylines = (polylines: [number, number][][]) => {
  lines.walks = polylines.filter((points) => points.length).map((points) => ({
    coords: points,
    color: paperActive.value ? paperInk() : legacyBlueActive.value ? '#245EDC' : '#38bdf8',
    width: 3,
    opacity: 0.85,
    dash: dashOf(paperActive.value ? '1 7' : '8 6', 3),
    square: legacyBlueActive.value,
  }))
  syncLines('walks')
  const bounds = linesBounds(lines.walks, lines.shapes)
  if (bounds && map.value && mapStore.fitWalkingPolylines && settingsStore.autoFitMap) {
    fitTo(bounds)
    mapStore.fitWalkingPolylines = false
    hasFittedForContent = true
  }
}

watch([walkingPolylines, arcadeActive, legacyBlueActive, paperActive, isDarkMode], ([polylines]) => {
  renderWalkingPolylines(polylines as [number, number][][])
}, {deep: true})


const renderHighlightedStops = () => {
  const m = map.value
  if (!m) return
  clearMarkers(highlightMarkers)
  highlightMarkers = []
  const selectedId = currentlyHighlightedStopId.value
  for (const {stopId, color} of highlightedStops.value as HighlightedStop[]) {
    if (stopId === selectedId) continue
    const stop = stopData.get(stopId)
    if (!stop) continue
    const highlight = htmlMarker(stop, makeHighlightIcon(color, themeOpts()), {
      zIndex: color === 'green' ? 1200 : color === 'red' ? 1000 : color === 'purple' ? 1100 : color === 'amber' ? 1050 : 800,
    })
    tooltip?.bind(highlight, stop.name, 'stop-name-tooltip', 14)
    onMarkerClick(highlight, () => router.push({name: 'stop', params: {stopId}}))
    highlightMarkers.push(highlight.addTo(m))
  }
}

watch([highlightedStops, currentlyHighlightedStopId, arcadeActive, legacyBlueActive, paperActive], renderHighlightedStops, {deep: true})


const renderVehicles = (vehicles: DisplayVehicle[]) => {
  const m = map.value
  if (!m) return

  clearMarkers(vehicleMarkers)
  vehicleMarkers = []
  for (const vehicle of vehicles) {
    if (vehicle.latitude <= 0 || vehicle.longitude <= 0) continue

    const resolvedColor = routeColorsCache.get(vehicle.trip_id) || vehicle.route_color
    if (!resolvedColor) continue

    const showStopInfo = selectedVehicleId.value === vehicle.id
    const faded = vehicle.faded && !showStopInfo
    const busHtml = getVehicleMarkerHtml(vehicle, resolvedColor, showStopInfo, themeOpts(), t)
    // Scaled around the icon anchor so the bus stays where it is.
    const markerHtml = faded ? `<div style="transform:scale(${FADED_VEHICLE_SCALE});transform-origin:18px 18px;">${busHtml}</div>` : busHtml
    const marker = htmlMarker({lat: vehicle.latitude, lng: vehicle.longitude}, {
      html: markerHtml,
      size: [36, 36],
      anchor: [18, 18],
    }, {
      zIndex: showStopInfo ? 5600 : vehicle.faded ? 4000 : 5000,
      opacity: faded ? FADED_VEHICLE_OPACITY : 1,
    })

    onMarkerClick(marker, () => {
      selectedVehicleId.value = selectedVehicleId.value === vehicle.id ? null : vehicle.id
    })

    vehicleMarkers.push(marker.addTo(m))
  }
}

watch([vehiclesToDisplay, () => route.name, selectedVehicleId, arcadeActive, legacyBlueActive, paperActive, showVehicleExtras], ([vehicles]) => {
  renderVehicles(vehicles as DisplayVehicle[])
}, {deep: true})

watch(() => route.name, () => {
  selectedVehicleId.value = null
})

watch([rideAs, arcadeActive, legacyBlueActive, paperActive], () => {
  if (userDot.value) setMarkerIcon(userDot.value, userIcon())
  syncAccuracy()
})

watch(preciseLocation, () => beginLocationWatch(preciseLocation.value))

watch(centerOnUser, (shouldCenter) => {
  if (!shouldCenter) return

  const userLocation = userDot.value ? markerLatLng(userDot.value) : null
  if (!userLocation) return

  flyToVisible(userLocation, DEFAULT_ZOOM)
  centerOnUser.value = false
})

watch(flyToLocation, (loc) => {
  if (!loc || !map.value) return
  flyToVisible(loc, DEFAULT_ZOOM)
  flyToLocation.value = null
})

const placeTooltipHtml = (label: string) => {
  const parts = label.split(',').map((p: string) => p.trim())
  const main = parts[0] ?? label
  const sub = parts.slice(1).join(', ')
  return sub
    ? `<div class="pin-tip-main">${main}</div><div class="pin-tip-sub">${sub}</div>`
    : `<div class="pin-tip-main">${main}</div>`
}

const placePin = (loc: { lat: number, lng: number, label: string }, color: string | undefined, onDragged: (lat: number, lng: number) => void) => {
  const marker = htmlMarker(loc, makePinIcon(themeOpts(), color), {zIndex: 9000, draggable: true})
  attachPlannerDragLift(marker)
  tooltip?.bind(marker, placeTooltipHtml(loc.label), 'pin-tooltip', 30)
  marker.on('dragend', () => {
    const {lat, lng} = marker.getLngLat()
    onDragged(lat, lng)
  })
  return marker
}

watch([pinnedLocation, arcadeActive, legacyBlueActive, paperActive], ([loc]) => {
  if (pinMarker.value) {
    tooltip?.forget(pinMarker.value)
    pinMarker.value.remove()
    pinMarker.value = undefined
  }
  if (!loc || !map.value) return
  pinMarker.value = placePin(loc as { lat: number, lng: number, label: string }, undefined,
    (lat, lng) => mapStore.setPinnedLocationDragged(lat, lng)).addTo(map.value)
})

watch([customOriginLocation, arcadeActive, legacyBlueActive, paperActive], ([loc]) => {
  if (originMarker.value) {
    tooltip?.forget(originMarker.value)
    originMarker.value.remove()
    originMarker.value = undefined
  }
  if (!loc || !map.value) return
  originMarker.value = placePin(loc as { lat: number, lng: number, label: string }, '#22c55e',
    (lat, lng) => mapStore.setCustomOriginLocationDragged(lat, lng)).addTo(map.value)
})

onUnmounted(() => {
  mapStore.clearPinnedLocation()
  mapStore.clearCustomOriginLocation()
  window.removeEventListener('resize', scheduleInvalidateMapSize)
  window.removeEventListener('orientationchange', scheduleInvalidateMapSize)
  window.removeEventListener('pageshow', retryLocationForStandaloneApp)
  document.removeEventListener('visibilitychange', retryLocationForStandaloneApp)
  window.visualViewport?.removeEventListener('resize', scheduleInvalidateMapSize)

  if (resizeRaf) cancelAnimationFrame(resizeRaf)
  if (stopRefreshFrame) cancelAnimationFrame(stopRefreshFrame)
  cancelAnimationFrame(glideFrame)
  if (resizeTimer) clearTimeout(resizeTimer)
  if (locationRetryTimer) clearTimeout(locationRetryTimer)

  stopLocate()
  map.value?.remove()
})

defineExpose({
  zoomBy: (delta: number) => map.value?.zoomTo(map.value.getZoom() + delta),
  focus: (lat: number, lng: number, zoom = DEFAULT_ZOOM) => {
    hasFittedForContent = true
    map.value?.jumpTo({center: [lng, lat], zoom})
  },
  fitShapes: (paddingTopLeft: [number, number], paddingBottomRight: [number, number]): boolean => {
    const bounds = linesBounds(lines.shapes)
    if (!map.value || !bounds) return false
    map.value.fitBounds(bounds, {
      padding: {left: paddingTopLeft[0], top: paddingTopLeft[1], right: paddingBottomRight[0], bottom: paddingBottomRight[1]},
      maxZoom: FIT_MAX_ZOOM,
      animate: false,
    })
    hasFittedForContent = true
    return true
  },
  invalidate: invalidateMapSize,
  maplibre: () => map.value,
})
</script>

<template>
  <div ref="mapContainer" class="map-container"></div>
</template>
