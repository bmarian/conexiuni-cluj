<script setup lang="ts">
import {LngLatBounds, Map as MapLibreMap, type Marker} from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import {storeToRefs} from 'pinia'
import {onBeforeUnmount, onMounted, ref, watch} from 'vue'
import type {HiddenStation} from '@/components/admin/types.ts'
import {useUserStore} from '@/stores/user.ts'
import {makeHighlightIcon} from '@/utils/mapIcons.ts'
import {FLAT_MAP_OPTIONS, htmlMarker, onElementClick} from '@/utils/mapMarkers.ts'
import {mapCreditsHTML, mapStyle} from '@/utils/mapStyles.ts'
import '@/utils/mapWorker.ts'

const FIT_PADDING_PX = 48
const STOP_ZOOM = 16

const props = defineProps<{stations: HiddenStation[]; focused: HiddenStation | null}>()
const emit = defineEmits<{pick: [station: HiddenStation]}>()

const {isDarkMode} = storeToRefs(useUserStore())
const container = ref<HTMLElement | null>(null)
let map: MapLibreMap | null = null
let markers: Marker[] = []

function drawMarkers() {
  markers.forEach((m) => m.remove())
  markers = props.stations.map((station) => {
    const marker = htmlMarker({lat: station.lat, lng: station.lon}, makeHighlightIcon(station === props.focused ? 'red' : 'gray')).addTo(map!)
    marker.getElement().title = station.name
    onElementClick(marker.getElement(), () => emit('pick', station))
    return marker
  })
}

function showAll() {
  const bounds = new LngLatBounds()
  props.stations.forEach((s) => bounds.extend([s.lon, s.lat]))
  map?.fitBounds(bounds, {padding: FIT_PADDING_PX, maxZoom: STOP_ZOOM, duration: 0})
}

onMounted(() => {
  map = new MapLibreMap({
    container: container.value!,
    style: mapStyle(isDarkMode.value),
    attributionControl: {compact: true, customAttribution: mapCreditsHTML()},
    ...FLAT_MAP_OPTIONS,
  })
  map.touchZoomRotate.disableRotation()
  drawMarkers()
  showAll()
})

onBeforeUnmount(() => map?.remove())

watch(
  () => props.stations,
  () => {
    drawMarkers()
    showAll()
  },
)

watch(
  () => props.focused,
  (station) => {
    drawMarkers()
    if (station) map?.flyTo({center: [station.lon, station.lat], zoom: STOP_ZOOM})
  },
)

watch(isDarkMode, (dark) => map?.setStyle(mapStyle(dark), {diff: true}))
</script>

<template>
  <div ref="container" class="stops-map"></div>
</template>

<style scoped>
.stops-map {
  height: 18rem;
  margin: 8px 16px;
  border-radius: 12px;
  overflow: hidden;
}
</style>
