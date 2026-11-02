<script setup lang="ts">
import {LngLatBounds, type Marker} from 'maplibre-gl'
import {ref, watch} from 'vue'
import type {HiddenStation} from '@/components/admin/types.ts'
import {ADMIN_MAP_PADDING_PX, useAdminMap} from '@/composables/useAdminMap.ts'
import {makeHighlightIcon} from '@/utils/mapIcons.ts'
import {htmlMarker, onElementClick} from '@/utils/mapMarkers.ts'

const STOP_ZOOM = 16

const props = defineProps<{stations: HiddenStation[]; focused: HiddenStation | null}>()
const emit = defineEmits<{pick: [station: HiddenStation]}>()

const container = ref<HTMLElement | null>(null)
let markers: Marker[] = []
const map = useAdminMap(container, () => {
  drawMarkers()
  showAll()
})

function drawMarkers() {
  markers.forEach((m) => m.remove())
  markers = props.stations.map((station) => {
    const marker = htmlMarker({lat: station.lat, lng: station.lon}, makeHighlightIcon(station === props.focused ? 'red' : 'gray')).addTo(map()!)
    marker.getElement().title = station.name
    onElementClick(marker.getElement(), () => emit('pick', station))
    return marker
  })
}

function showAll() {
  const bounds = new LngLatBounds()
  props.stations.forEach((s) => bounds.extend([s.lon, s.lat]))
  map()?.fitBounds(bounds, {padding: ADMIN_MAP_PADDING_PX, maxZoom: STOP_ZOOM, duration: 0})
}

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
    if (station) map()?.flyTo({center: [station.lon, station.lat], zoom: STOP_ZOOM})
  },
)
</script>

<template>
  <div ref="container" class="admin-map"></div>
</template>
