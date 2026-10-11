<script setup lang="ts">
import {computed} from 'vue'
import {useVehicleStream} from '@/composables/useVehicleStream.ts'
import {useFollowsStore} from '@/stores/follows.ts'
import {useJourneyStore} from '@/stores/journey.ts'
import FollowTracker from '@/components/FollowTracker.vue'
import JourneyTracker from '@/components/JourneyTracker.vue'

const follows = useFollowsStore()
const journey = useJourneyStore()

const tripIds = computed(() => [...new Set([...follows.follows.map((f) => f.tripId), ...journey.tripIds])].sort())
const {vehiclesByTrip} = useVehicleStream(tripIds)
</script>

<template>
  <FollowTracker v-for="f in follows.follows" :key="f.id" :followed="f" :vehicles="vehiclesByTrip"/>
  <JourneyTracker v-if="journey.journey" :key="journey.journey.id" :vehicles="vehiclesByTrip"/>
</template>
