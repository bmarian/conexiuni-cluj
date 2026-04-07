<script setup lang="ts">
import {onMounted, onUnmounted, ref, shallowRef, watch} from 'vue'
import L, { Map } from 'leaflet'
import 'leaflet/dist/leaflet.css'

import { GeoSearchControl, OpenStreetMapProvider } from 'leaflet-geosearch'
import 'leaflet-geosearch/dist/geosearch.css'
import {useUserStore} from "@/stores/user.ts";
import {storeToRefs} from "pinia";

const userStore = useUserStore()
const {isDarkMode} = storeToRefs(userStore)
const mapContainer = ref()
const map = shallowRef<Map>()

const mapInit = (lat: number, lon: number, zoom: number) => {
  map.value = L.map(mapContainer.value).setView([lat, lon], zoom)

  L.tileLayer(`https://{s}.basemaps.cartocdn.com/${isDarkMode.value ? 'dark_all' : 'light_all'}/{z}/{x}/{y}{r}.png`, {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>, &copy; <a href="https://carto.com/attributions">CARTO</a>',
    maxZoom: 20
  }).addTo(map.value)

  const provider = new OpenStreetMapProvider()
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-expect-error
  const searchControl = new GeoSearchControl({
    provider,
    showMarker: true,
    retainZoomLevel: false,
    animateZoom: true,
    autoClose: true,
    searchLabel: 'Enter address...',
    keepResult: true
  })
  map.value.addControl(searchControl)

  const myCustomIcon = L.icon({
    // Replace this URL with the path to your own icon (e.g., '/my-icon.png')
    iconUrl: 'https://cdn-icons-png.flaticon.com/512/252/252025.png',
    iconSize: [40, 40], // Size of the icon [width, height]
    iconAnchor: [20, 40], // Point of the icon which will correspond to marker's location
    popupAnchor: [0, -40] // Point from which the popup should open relative to the iconAnchor
  })

  L.marker([46.7704, 23.5914], { icon: myCustomIcon })
    .addTo(map.value)
    .bindPopup('This is my custom icon!')
}

onMounted(() => {
mapInit(46.7712, 23.6236, 13)
})

watch(isDarkMode, () => {
  if (map.value) {
    const mapCenter = map.value.getCenter()
    const lat = mapCenter.lat
    const lon = mapCenter.lng
    const zoom = map.value.getZoom()

    map.value.remove()
    mapInit(lat, lon, zoom)
  }
})

onUnmounted(() => {
  if (map.value) { map.value.remove() }
})
</script>

<template>
  <div ref="mapContainer" class="map-container"></div>
</template>

<style scoped lang="scss">
</style>
