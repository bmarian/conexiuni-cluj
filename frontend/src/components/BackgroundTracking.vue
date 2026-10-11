<script setup lang="ts">
import {computed} from 'vue'
import {useVehicleStream} from '@/composables/useVehicleStream.ts'
import {useFollowsStore} from '@/stores/follows.ts'
import {useJourneyStore} from '@/stores/journey.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import FollowTracker from '@/components/FollowTracker.vue'
import JourneyTracker from '@/components/JourneyTracker.vue'

const follows = useFollowsStore()
const journey = useJourneyStore()
const settings = useSettingsStore()

const followed = computed(() => (settings.busAlerts ? follows.follows : []))
const tripIds = computed(() => [...new Set([...followed.value.map((f) => f.tripId), ...journey.tripIds])].sort())
const {vehiclesByTrip} = useVehicleStream(tripIds)
</script>

<template>
  <FollowTracker v-for="f in followed" :key="f.id" :followed="f" :vehicles="vehiclesByTrip"/>
  <JourneyTracker v-if="journey.journey" :key="journey.journey.id" :vehicles="vehiclesByTrip"/>
</template>
