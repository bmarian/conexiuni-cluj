<script setup lang="ts">
const props = defineProps<{
  routeShortName: string
}>()
</script>

<template>
  <div>
    <h1>Route {{ routeShortName }}</h1>
  </div>
</template>
