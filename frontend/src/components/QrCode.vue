<script setup lang="ts">
import {computed} from 'vue'
import {encode} from 'uqr'

const props = defineProps<{ text: string; label?: string }>()

const qr = computed(() => {
  const {data, size} = encode(props.text, {border: 0})
  let path = ''
  data.forEach((row, y) => row.forEach((dark, x) => {
    if (dark) path += `M${x} ${y}h1v1h-1z`
  }))
  return {size, path}
})
</script>

<template>
  <svg :viewBox="`0 0 ${qr.size} ${qr.size}`" shape-rendering="crispEdges" role="img" :aria-label="label">
    <path :d="qr.path" fill="#0f172a"/>
  </svg>
</template>
