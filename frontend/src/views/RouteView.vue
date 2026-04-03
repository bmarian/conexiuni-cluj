<script setup lang="ts">
import {computed, onMounted, ref, watch} from "vue"
import type {Timetable} from "@/types/ctp.ts"
import {INCOMING_SUFFIX, OUTGOING_SUFFIX, type Route, type StopTime} from "@/types/tranzy.ts"
import {useUserStore} from "@/stores/user.ts"
import {storeToRefs} from "pinia"
import {closestStop} from "@/utils/geo.ts"
import RouteHeaderComponent from "@/components/RouteHeaderComponent.vue";

const props = defineProps<{
  routeShortName: string
}>()

const userStore = useUserStore()
const {currentLocation: userLocation} = storeToRefs(userStore)
const timetable = ref<Timetable>()
const stopTimes = ref<StopTime[]>()
const routeProperties = ref<Route>()
const errorState = ref(false)
const loading = ref(true)

onMounted(async () => {
  try {
    const timetableResponse = await fetch(`/api/timetable?route_short_name=${props.routeShortName}`)
    if (timetableResponse.ok) {
      timetable.value = await timetableResponse.json()
    }

    const stopTimesResponse = await fetch(`/api/stop_times?route_short_name=${props.routeShortName}`)
    if (stopTimesResponse.ok) {
      stopTimes.value = await stopTimesResponse.json()
    }

    const routePropertiesResponse = await fetch(`/api/routes?route_short_name=${props.routeShortName}`)
    if (routePropertiesResponse.ok) {
      const props = await routePropertiesResponse.json()
      routeProperties.value = props?.[0];
    }

  } catch (err) {
    errorState.value = true
    console.log(err)
  } finally {
    loading.value = false
  }
})

const closestStopToUserOutgoing = computed(() => {
  if (!userLocation.value || !stopTimes.value) return {}
  const outgoingStops = stopTimes.value?.filter(stop => stop.trip_id.includes(OUTGOING_SUFFIX))
  return closestStop(userLocation.value, outgoingStops)
})

const closestStopToUserIncoming = computed(() => {
  if (!userLocation.value || !stopTimes.value) return {}
  const incomingStops = stopTimes.value?.filter(stop => stop.trip_id.includes(INCOMING_SUFFIX))
  return closestStop(userLocation.value, incomingStops)
})

watch([closestStopToUserIncoming, closestStopToUserOutgoing], ([newIncoming, newOutgoing]) => {
  console.log(newIncoming, newOutgoing);
}, {immediate: true})

</script>

<template>
  <div>
    <div v-if="loading">Loading...</div>
    <div v-else>
      <route-header-component v-if="routeProperties" :route-properties="routeProperties"/>
      <pre>{{ timetable }}</pre>
      <pre>{{ stopTimes }}</pre>
    </div>
  </div>
</template>
