<script setup lang="ts">
import {computed} from 'vue'
import FavoriteMorph from '@/components/ui/FavoriteMorph.vue'
import {useFavoritesStore} from '@/stores/favorites.ts'
import {displayColor} from '@/utils/lineColor.ts'
import {network} from '@/utils/network.ts'

const {name, color, height = 28, minWidth = 40, corner = 8, plain = false} = defineProps<{
  name: string
  color?: string
  plain?: boolean
  height?: number
  minWidth?: number
  corner?: number
}>()

const favorites = useFavoritesStore()
const shown = computed(() => (plain ? color : displayColor(name, color ?? '')) || 'var(--md-outline)')
const favorite = computed(() => !plain && network.value ? favorites.favoriteLineNames.has(name) : undefined)
</script>

<template>
  <FavoriteMorph
    :subject="name"
    :color="shown"
    :favorite="favorite"
    class="line-badge"
    :style="{
      minHeight: `${height}px`,
      minWidth: `${minWidth}px`,
      borderRadius: `${corner}px`,
      fontSize: `${height * 0.45}px`,
    }"
  >
    <span>{{ name }}</span>
  </FavoriteMorph>
</template>

<style scoped>
.line-badge {
  flex-shrink: 0;
  padding: 0 6px;
  color: #fff;
  font-weight: 900;
  line-height: 1;
  white-space: nowrap;
}
</style>
