<script setup lang="ts">
import type {StationLook} from '@/utils/network.ts'
import ModeIcon from '@/components/icons/ModeIcon.vue'
import FavoriteMorph from '@/components/ui/FavoriteMorph.vue'

defineProps<{ look?: StationLook, corner?: number, favorite?: boolean }>()
</script>

<template>
  <FavoriteMorph
    :color="look?.color ?? 'var(--md-secondary-container)'"
    :favorite="favorite"
    class="station-icon"
    :class="{ 'is-plain': !look }"
    :style="{ borderRadius: corner === undefined ? '50%' : `${corner}px` }"
  >
    <ModeIcon :mode="look?.mode ?? 'bus'" class="station-icon-glyph"/>
  </FavoriteMorph>
</template>

<style scoped>
.station-icon {
  display: flex;
  flex-shrink: 0;
  color: white;
}

.station-icon.is-plain {
  color: var(--md-on-secondary-container);
}

.station-icon-glyph {
  width: 50%;
  height: 50%;
}
</style>
