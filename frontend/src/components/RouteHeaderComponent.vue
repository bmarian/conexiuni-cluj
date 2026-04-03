<script setup lang="ts">

import type {Route} from "@/types/tranzy.ts";

const props = defineProps<{
  routeProperties: Route
}>()

</script>

<template>
  <div class="p-4 shadow-sm rounded-lg">
    <div class="flex items-center gap-3">
      <div
        class="flex items-center justify-center min-w-[3rem] min-h-[3rem] px-2 rounded font-bold text-white shadow-sm"
        :style="{ backgroundColor: props.routeProperties.route_color }"
      >
        {{ props.routeProperties.route_short_name }}
      </div>
      <div>
        <div class="text-lg font-semibold">
          {{ $t('route.line', { name: props.routeProperties.route_short_name }) }}
        </div>
        <div class="text-sm font-medium text-slate-700 dark:text-slate-400">
          {{ props.routeProperties.route_long_name }}
        </div>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>

</style>
