<script setup lang="ts">
import type {StationLook} from '@/utils/network.ts'
import ModeIcon from '@/components/icons/ModeIcon.vue'

defineProps<{ look?: StationLook, corner?: number }>()
</script>

<template>
  <span class="station-icon" :class="{ 'is-plain': !look }" :style="{ backgroundColor: look?.color, borderRadius: corner === undefined ? '50%' : `${corner}px` }">
    <ModeIcon :mode="look?.mode ?? 'bus'" class="station-icon-glyph"/>
  </span>
</template>

<style scoped>
.station-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: white;
}

.station-icon.is-plain {
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}

.station-icon-glyph {
  width: 50%;
  height: 50%;
}
</style>
