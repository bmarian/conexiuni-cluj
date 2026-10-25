<script setup lang="ts">
import type {Route} from '@/types/tranzy.ts'
import {displayColor} from '@/utils/lineColor.ts'

defineProps<{routes: Pick<Route, 'route_id' | 'route_short_name' | 'route_color'>[]}>()
</script>

<template>
  <span v-if="routes.length" class="station-lines">
    <span v-for="line in routes" :key="line.route_id" class="station-line" :style="{backgroundColor: displayColor(line.route_short_name, line.route_color)}">
      {{ line.route_short_name }}
    </span>
  </span>
</template>

<style scoped>
.station-lines {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.station-line {
  min-width: 28px;
  padding: 1px 5px;
  border-radius: 5px;
  color: #fff;
  font-size: 11px;
  line-height: 16px;
  font-weight: 900;
  text-align: center;
}
</style>
