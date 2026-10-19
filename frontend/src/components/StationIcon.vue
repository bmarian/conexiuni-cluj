<script setup lang="ts">
import type {StationLook} from '@/utils/network.ts'
import ModeIcon from '@/components/icons/ModeIcon.vue'

defineProps<{ look?: StationLook }>()
</script>

<!-- The station's biggest mode in that mode's colour; size and corners come from the caller. -->
<template>
  <span class="station-icon shrink-0" :class="{ 'is-plain': !look }" :style="look ? { backgroundColor: look.color } : undefined">
    <ModeIcon :mode="look?.mode ?? 'bus'" class="station-icon-glyph"/>
  </span>
</template>

<style scoped>
.station-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
}

.station-icon.is-plain {
  background: var(--color-emerald-100);
  color: var(--color-emerald-600);
}

.station-icon-glyph {
  width: 50%;
  height: 50%;
}
</style>
