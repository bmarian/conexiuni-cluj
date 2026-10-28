<script setup lang="ts">
import type {StationLook} from '@/utils/network.ts'
import ModeIcon from '@/components/icons/ModeIcon.vue'
import FavoriteMorph from '@/components/ui/FavoriteMorph.vue'
import {FavoriteIcon} from '@/utils/icons.ts'

defineProps<{look?: StationLook; corner?: number; favorite?: boolean; heart?: boolean}>()
</script>

<template>
  <FavoriteMorph
    :color="heart ? 'var(--md-rose)' : (look?.color ?? 'var(--md-secondary-container)')"
    :favorite="favorite"
    class="station-icon"
    :class="{'is-plain': !look && !heart}"
    :style="{borderRadius: corner === undefined ? '50%' : `${corner}px`}"
  >
    <FavoriteIcon v-if="heart" class="station-icon-glyph" />
    <ModeIcon v-else :mode="look?.mode ?? 'bus'" class="station-icon-glyph" />
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
  fill: currentColor;
}
</style>
