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
import {useRouter} from "vue-router";
import {useI18n} from "vue-i18n";
import {type DisplayShape, useMapStore} from "@/stores/map.ts";
import type {ShapePoint} from "@/types/map.ts";

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapStore = useMapStore()
const {shapesToDisplay, centerOnUser, zoomOut, vehiclesToDisplay} = storeToRefs(mapStore)
const router = useRouter()
const {t, locale} = useI18n()
const mapContainer = ref()

const map = shallowRef<L.Map>()
const stopGroup = shallowRef<L.FeatureGroup>()
const currentTileLayer = shallowRef<L.TileLayer>()
const userDot = shallowRef<L.Marker>()
const accuracyCircle = shallowRef<L.Circle>()
const shapeLayerGroup = shallowRef<L.FeatureGroup>()

let isFirstLocationHandle = true
const DEFAULT_ZOOM = 16
const STOP_ZOOM_THRESHOLD = 16

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
    <div class="text-rose-500 w-8 h-8 drop-shadow-lg -mt-2 text-fuchsia-900 dark:text-orange-500">
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
  handleZoomVisibility()
}

const stopsInit = async () => {
  const stops = await apiRequest('stops') as Stop[]
  if (!Array.isArray(stops) || !stops.length || !stopGroup.value) return

  const stopIcon = L.divIcon({
    className: 'bg-transparent border-none',
    html: `
          <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-md z-20 bg-fuchsia-500">
                <svg fill="currentColor" width="24" height="24" xmlns="http://www.w3.org/2000/svg" fill-rule="evenodd" clip-rule="evenodd"><path d="M10 23h-1c-.552 0-1-.448-1-1v-1c-.53 0-1.039-.211-1.414-.586s-.586-.884-.586-1.414v-6c-.552 0-1-.448-1-1v-3c0-.552.448-1 1-1v-2c0-1.657 1.343-3 3-3h11c1.657 0 3 1.343 3 3v2c.552 0 1 .448 1 1v3c0 .552-.448 1-1 1v6c0 .53-.211 1.039-.586 1.414s-.884.586-1.414.586v1c0 .552-.448 1-1 1h-1c-.552 0-1-.448-1-1v-1h-7v1c0 .552-.448 1-1 1zm-5 0h-5v-1h2v-15h-1c-.552 0-1-.448-1-1v-4c0-.552.448-1 1-1h3c.552 0 1 .448 1 1v4c0 .552-.448 1-1 1h-1v15h2v1zm4.25-6.75c.69 0 1.25.56 1.25 1.25s-.56 1.25-1.25 1.25-1.25-.56-1.25-1.25.56-1.25 1.25-1.25zm10.5 0c.69 0 1.25.56 1.25 1.25s-.56 1.25-1.25 1.25-1.25-.56-1.25-1.25.56-1.25 1.25-1.25zm-3.25.75h-4c-.276 0-.5.224-.5.5s.224.5.5.5h4c.276 0 .5-.224.5-.5s-.224-.5-.5-.5zm4.5-10.5c0-.276-.224-.5-.5-.5h-12c-.276 0-.5.224-.5.5v7s.626 1 6.528 1c5.903 0 6.472-1 6.472-1v-7z"/></svg>
          </div>
        `,
    iconSize: [24, 24],
    iconAnchor: [12, 24],
    popupAnchor: [0, -24]
  })

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
  }
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
  // Move map to user location once
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

  const groupedStarts = new Map<string, {
    lat: number,
    lng: number,
    routes: { name: string, color: string }[]
  }>()

  const fallbackColors = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ec4899', '#06b6d4']
  let fallbackIndex = 0
  const assignedFallbacks = new Map<string, string>()

  const getRouteColor = (color: string | undefined, routeName: string) => {
    let finalColor = color

    if (finalColor && !finalColor.startsWith('#')) {
      finalColor = `#${finalColor}`
    }

    if (!finalColor || finalColor === '#000' || finalColor === '#000000') {
      if (!assignedFallbacks.has(routeName)) {
        assignedFallbacks.set(routeName, fallbackColors[fallbackIndex % fallbackColors.length]!)
        fallbackIndex++
      }
      return assignedFallbacks.get(routeName)!
    }

    return finalColor
  }

  for (let i = 0; i < newShapes.length; i++) {
    const [displayShape, shapeData]: [DisplayShape, ShapePoint[]] = newShapes[i]!
    if (!Array.isArray(shapeData) || shapeData.length === 0) continue

    const latLngs: L.LatLngTuple[] = shapeData.map(sd => [sd.shape_pt_lat, sd.shape_pt_lon])
    const routeName = displayShape.route_short_name || `unknown-${i}`
    const routeColor = getRouteColor(displayShape.route_color, routeName)

    L.polyline(latLngs, {
      color: routeColor,
      weight: 6,
      opacity: 0.9,
      smoothFactor: 1.5,
      lineJoin: 'round',
      lineCap: 'round'
    }).addTo(shapeLayerGroup.value!)

    const startPoint = latLngs[0]
    if (startPoint && displayShape.route_short_name) {
      const key = `${startPoint[0].toFixed(4)},${startPoint[1].toFixed(4)}`

      if (!groupedStarts.has(key)) {
        groupedStarts.set(key, {lat: startPoint[0], lng: startPoint[1], routes: []})
      }

      const existing = groupedStarts.get(key)!
      if (!existing.routes.some(r => r.name === displayShape.route_short_name)) {
        existing.routes.push({name: displayShape.route_short_name, color: routeColor})
      }
    }

    const endPoint = latLngs[latLngs.length - 1]
    if (endPoint) {
      const endMarkerIcon = L.divIcon({
        className: 'bg-transparent border-none',
        html: `
          <div class="flex items-center justify-center w-8 h-8 rounded-full border-2 border-white shadow-md z-20"
               style="background-color: ${routeColor};">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
            </svg>
          </div>
        `,
        iconSize: [32, 32],
        iconAnchor: [16, 16]
      })

      L.marker(endPoint, {icon: endMarkerIcon}).addTo(shapeLayerGroup.value!)
    }
  }

  groupedStarts.forEach((data) => {
    const routesHtml = data.routes.map(r =>
      `<div style="background-color: ${r.color};"
            class="flex items-center justify-center min-w-[30px] h-[30px] px-1.5 rounded-full text-white text-xs font-bold shadow-md border-2 border-white">
        ${r.name}
      </div>`
    ).join('')

    const startMarkerIcon = L.divIcon({
      className: 'bg-transparent border-none !overflow-visible',
      html: `
        <div class="absolute flex flex-row items-center justify-center gap-1 whitespace-nowrap" style="transform: translate(-50%, -50%);">
          ${routesHtml}
        </div>
      `,
      iconSize: [0, 0],
      iconAnchor: [0, 0]
    })

    L.marker([data.lat, data.lng], {icon: startMarkerIcon}).addTo(shapeLayerGroup.value!)
  })

  if (zoomOut.value && shapeLayerGroup.value.getLayers().length > 0) {
    map.value.fitBounds(shapeLayerGroup.value.getBounds(), {padding: [50, 50]})
    zoomOut.value = false
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
