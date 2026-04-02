<script setup lang="ts">

import type {Route} from "@/types/tranzy.ts";

const props = defineProps<{
  routeProperties: Route
}>()

</script>

<template>
  <div>
    <pre>{{props.routeProperties}}</pre>
  </div>
</template>

<style lang="scss" scoped>

</style>
