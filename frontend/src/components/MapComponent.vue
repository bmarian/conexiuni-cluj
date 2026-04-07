<script setup lang="ts">
import {onMounted, onUnmounted, ref, shallowRef, watch} from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import {GeoSearchControl, OpenStreetMapProvider} from 'leaflet-geosearch'
import 'leaflet-geosearch/dist/geosearch.css'
import 'leaflet.markercluster'
import 'leaflet.markercluster/dist/MarkerCluster.css'
import 'leaflet.markercluster/dist/MarkerCluster.Default.css'
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";
import {apiRequest} from "@/utils/request_cache.ts";
import type {Stop} from "@/types/tranzy.ts";
import {useRouter} from "vue-router";
import {useI18n} from "vue-i18n";

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapContainer = ref()
const map = shallowRef<L.Map>()
const stopGroup = shallowRef<L.MarkerClusterGroup>()
const currentTileLayer = shallowRef<L.TileLayer>()
const router = useRouter()
const { t, locale } = useI18n()

const mapInit = (lat: number, lon: number, zoom: number) => {
  map.value = L.map(mapContainer.value).setView([lat, lon], zoom)

  currentTileLayer.value = L.tileLayer(`https://{s}.basemaps.cartocdn.com/${isDarkMode.value ? 'dark_all' : 'light_all'}/{z}/{x}/{y}{r}.png`, {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>, &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20
  }).addTo(map.value)

  // TODO: i18n, limit search to Cluj
  const provider = new OpenStreetMapProvider()
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-expect-error
  const searchControl = new GeoSearchControl({
    provider,
    showMarker: true,
    retainZoomLevel: false,
    animateZoom: true,
    autoClose: true,
    searchLabel: t('Search location...'),
    keepResult: true
  })
  map.value.addControl(searchControl)

  stopGroup.value = L.markerClusterGroup().addTo(map.value)
}

const stopsInit = async () => {
  const stops = await apiRequest('stops') as Stop[]
  if (!Array.isArray(stops) || !stops.length || !stopGroup.value) return

  const stopIcon = L.divIcon({
    html: `
    <div class="text-fuchsia-900 dark:text-fuchsia-500">
        <svg fill="currentColor" width="24" height="24" xmlns="http://www.w3.org/2000/svg" fill-rule="evenodd" clip-rule="evenodd"><path d="M10 23h-1c-.552 0-1-.448-1-1v-1c-.53 0-1.039-.211-1.414-.586s-.586-.884-.586-1.414v-6c-.552 0-1-.448-1-1v-3c0-.552.448-1 1-1v-2c0-1.657 1.343-3 3-3h11c1.657 0 3 1.343 3 3v2c.552 0 1 .448 1 1v3c0 .552-.448 1-1 1v6c0 .53-.211 1.039-.586 1.414s-.884.586-1.414.586v1c0 .552-.448 1-1 1h-1c-.552 0-1-.448-1-1v-1h-7v1c0 .552-.448 1-1 1zm-5 0h-5v-1h2v-15h-1c-.552 0-1-.448-1-1v-4c0-.552.448-1 1-1h3c.552 0 1 .448 1 1v4c0 .552-.448 1-1 1h-1v15h2v1zm4.25-6.75c.69 0 1.25.56 1.25 1.25s-.56 1.25-1.25 1.25-1.25-.56-1.25-1.25.56-1.25 1.25-1.25zm10.5 0c.69 0 1.25.56 1.25 1.25s-.56 1.25-1.25 1.25-1.25-.56-1.25-1.25.56-1.25 1.25-1.25zm-3.25.75h-4c-.276 0-.5.224-.5.5s.224.5.5.5h4c.276 0 .5-.224.5-.5s-.224-.5-.5-.5zm4.5-10.5c0-.276-.224-.5-.5-.5h-12c-.276 0-.5.224-.5.5v7s.626 1 6.528 1c5.903 0 6.472-1 6.472-1v-7z"/></svg>
    </div>
    `,
    className: 'bg-transparent border-none',
    iconSize: [28, 28],
    iconAnchor: [14, 28],
    popupAnchor: [0, -28]
  })

  const markers = []
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

    markers.push(marker)
  }
  stopGroup.value.addLayers(markers)
}

onMounted(() => {
  mapInit(46.7712, 23.6236, 13)
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

onUnmounted(() => {
  if (map.value) {
    map.value.remove()
  }
})
</script>

<template>
  <div ref="mapContainer" class="map-container"></div>
</template>

<style scoped lang="scss">
</style>
