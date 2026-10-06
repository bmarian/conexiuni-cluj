<script lang="ts">
export type WatchMapLabel = { id: string; lat: number; lng: number; text: string; live: boolean; rank: number }
export type WatchMapFocus = { lat: number; lng: number; zoom?: number }
</script>

<script setup lang="ts">
import {nextTick, onMounted, onUnmounted, ref, watch} from 'vue'
import L from 'leaflet'
import {storeToRefs} from 'pinia'
import MapComponent from '@/components/MapComponent.vue'
import {useMapStore} from '@/stores/map.ts'

// Each bezel click scrolls the document 128px and sends nothing else, so the map sits
// over a tall empty page and turns those scrolls into zoom steps.
const BEZEL_STEP_PX = 128
// Keeps a fitted route inside the round screen, clear of the title and the buttons.
const FIT_SIDE = 0.12
const FIT_TOP = 0.2
const FIT_BOTTOM = 0.24

const props = withDefaults(defineProps<{
  active?: boolean
  labels?: WatchMapLabel[]
  focus?: WatchMapFocus | null
}>(), {active: true, labels: () => [], focus: null})

const {shapesToDisplay} = storeToRefs(useMapStore())
const map = ref<InstanceType<typeof MapComponent> | null>(null)

let middle = 0
let framed = false

function recenter() {
  middle = Math.round((document.documentElement.scrollHeight - window.innerHeight) / 2)
  window.scrollTo(0, middle)
}

function onScroll() {
  if (!props.active) return
  const delta = window.scrollY - middle
  if (Math.abs(delta) < BEZEL_STEP_PX / 4) return
  map.value?.zoomBy(Math.sign(delta) * Math.max(1, Math.round(Math.abs(delta) / BEZEL_STEP_PX)))
  recenter()
}

function frame() {
  if (framed || !map.value) return
  if (props.focus) {
    map.value.focus(props.focus.lat, props.focus.lng, props.focus.zoom)
    framed = true
  } else if (shapesToDisplay.value.length) {
    const w = window.innerWidth, h = window.innerHeight
    framed = map.value.fitShapes([w * FIT_SIDE, h * FIT_TOP], [w * FIT_SIDE, h * FIT_BOTTOM])
  }
}

let labelLayer: L.LayerGroup | null = null
let labelMarkers: Array<{ label: WatchMapLabel; marker: L.Marker }> = []

function drawLabels() {
  const leaflet = map.value?.leaflet()
  if (!leaflet) return
  labelLayer ??= L.layerGroup().addTo(leaflet)
  labelLayer.clearLayers()
  labelMarkers = props.labels.map((label) => {
    const text = document.createElement('span')
    text.className = label.live ? 'wt-map-label is-live' : 'wt-map-label'
    text.textContent = label.text
    const marker = L.marker([label.lat, label.lng], {
      icon: L.divIcon({className: 'wt-map-label-anchor', html: text, iconSize: [0, 0]}),
      interactive: false,
      zIndexOffset: 4000,
    }).addTo(labelLayer!)
    return {label, marker}
  })
  cullLabels()
}

// Soonest first. A label that would cover one already placed stays hidden until
// zooming in makes room for it.
function cullLabels() {
  const placed: DOMRect[] = []
  for (const {marker} of [...labelMarkers].sort((a, b) => a.label.rank - b.label.rank)) {
    const el = marker.getElement()?.firstElementChild as HTMLElement | null | undefined
    if (!el) continue
    const box = el.getBoundingClientRect()
    const free = !placed.some((o) =>
      box.left < o.right + 2 && box.right + 2 > o.left && box.top < o.bottom + 2 && box.bottom + 2 > o.top)
    el.style.visibility = free ? '' : 'hidden'
    if (free) placed.push(box)
  }
}

watch(() => props.labels, drawLabels, {deep: true})
watch(() => props.focus, frame)
watch(shapesToDisplay, frame, {flush: 'post'})

watch(() => props.active, async (active) => {
  if (!active) return
  await nextTick()
  map.value?.invalidate()
  recenter()
  cullLabels()
})

onMounted(() => {
  recenter()
  window.addEventListener('scroll', onScroll, {passive: true})
  window.addEventListener('resize', recenter)
  map.value?.leaflet()?.on('zoomend moveend', cullLabels)
  frame()
  drawLabels()
})

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('resize', recenter)
})
</script>

<template>
  <MapComponent v-show="active" ref="map" class="wt-map"/>
  <div v-show="active" class="wt-map-sink" aria-hidden="true"></div>
</template>

<style scoped>
.wt-map {
  position: fixed;
  inset: 0;
  z-index: 10;
  height: 100dvh;
}

.wt-map-sink {
  height: 3000vh;
}
</style>
