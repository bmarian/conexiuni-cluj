<script setup lang="ts">
import {onMounted, ref} from "vue";

const props = defineProps<{
  routeShortName: string
}>()

const timetable = ref<any>(null);
const loading = ref(true);
const error = ref<string | null>(null);

onMounted(async () => {
  try {
    const response = await fetch(`/api/timetable?route_short_name=${props.routeShortName}`);
    if (response.ok) {
      timetable.value = await response.json();
    } else {
      error.value = `Request failed with status ${response.status}`;
    }

  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Unknown error';
  } finally {
    loading.value = false;
  }
});

</script>

<template>
  <div>
    <h1>Timetable {{ props.routeShortName }}:</h1>
    <div v-if="loading">Loading...</div>
    <div v-else-if="error">Error: {{ error }}</div>
    <pre v-else>{{ timetable }}</pre>
  </div>
</template>
