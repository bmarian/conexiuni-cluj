<script setup lang="ts">
import {onMounted, onUnmounted, ref} from 'vue'
import {useRoute} from 'vue-router'
import {storeToRefs} from 'pinia'
import MapComponent from '@/components/MapComponent.vue'
import {useUserStore} from '@/stores/user.ts'
import {useStopsApi} from '@/composables/useStopsApi.ts'

// Each bezel click scrolls the document 128px and sends nothing else, so the map sits
// over a tall empty page and turns those scrolls into zoom steps.
const BEZEL_STEP_PX = 128

const route = useRoute()
const {userLocation} = storeToRefs(useUserStore())
const {stops} = useStopsApi()
const map = ref<InstanceType<typeof MapComponent> | null>(null)

let middle = 0

function recenter() {
  middle = Math.round((document.documentElement.scrollHeight - window.innerHeight) / 2)
  window.scrollTo(0, middle)
}

function onScroll() {
  const delta = window.scrollY - middle
  if (Math.abs(delta) < BEZEL_STEP_PX / 4) return
  map.value?.zoomBy(Math.sign(delta) * Math.max(1, Math.round(Math.abs(delta) / BEZEL_STEP_PX)))
  recenter()
}

onMounted(() => {
  recenter()
  window.addEventListener('scroll', onScroll, {passive: true})
  window.addEventListener('resize', recenter)

  if (route.name === 'stop') {
    const stop = stops.value.find((s) => String(s.stop_id) === route.params.stopId)
    if (stop) map.value?.focus(stop.stop_lat, stop.stop_lon)
  } else if (route.name === 'home' && userLocation.value) {
    map.value?.focus(userLocation.value.latitude, userLocation.value.longitude)
  }
})

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('resize', recenter)
})
</script>

<template>
  <MapComponent ref="map" class="wt-map"/>
  <div class="wt-map-sink" aria-hidden="true"></div>
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
