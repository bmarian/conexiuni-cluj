<script setup lang="ts">
import LiveDot from '@/components/ui/LiveDot.vue'

defineProps<{text: string}>()
</script>

<template>
  <div class="departures-label">
    <LiveDot />
    <h2 class="text-title-small text-on-surface-variant">{{ text }}</h2>
  </div>
</template>

<style scoped>
.departures-label {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
}
</style>
