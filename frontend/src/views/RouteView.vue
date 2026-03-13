<script setup lang="ts">
import {onMounted, ref} from "vue";
import type {Timetable} from "@/types/ctp.ts";
import type {StopTime} from "@/types/tranzy.ts";

const props = defineProps<{
  routeShortName: string
}>()

const timetable = ref<Timetable>();
const stopTimes = ref<StopTime[]>();
const loading = ref(true);

onMounted(async () => {
  try {
    const timetableResponse = await fetch(`/api/timetable?route_short_name=${props.routeShortName}`);
    if (timetableResponse.ok) {
      timetable.value = await timetableResponse.json();
    }

    const stopTimesResponse = await fetch(`/api/stop_times?route_short_name=${props.routeShortName}`);
    if (stopTimesResponse.ok) {
      stopTimes.value = await stopTimesResponse.json();
    }

  } catch (err) {
    console.log(err);
  } finally {
    loading.value = false;
  }
});

</script>

<template>
  <div>
    <div v-if="loading">Loading...</div>
    <div v-else>
      <h1>Timetable {{ props.routeShortName }}:</h1>
      <pre>{{ timetable }}</pre>

      <h1>Stop Times {{ props.routeShortName }}:</h1>
      <pre>{{ stopTimes }}</pre>
    </div>
  </div>
</template>
