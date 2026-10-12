<script setup lang="ts">
import {computed} from 'vue'
import {useLive} from '@/composables/useLive.ts'
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
const stopIds = computed(() => [...new Set([
  ...followed.value.map((f) => f.stopId),
  ...(journey.journey?.legs ?? []).map((leg) => leg.stops[0]?.id).filter((id): id is number => id !== undefined),
])].sort((a, b) => a - b))
const {rows, asOfMs} = useLive(stopIds, tripIds)
</script>

<template>
  <FollowTracker v-for="f in followed" :key="f.id" :followed="f" :rows="rows" :as-of-ms="asOfMs"/>
  <JourneyTracker v-if="journey.journey" :key="journey.journey.id" :rows="rows" :as-of-ms="asOfMs"/>
</template>
