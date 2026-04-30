<script setup lang="ts">
import { ref, watchEffect } from 'vue'

interface WeatherIconProps {
  slug: string
  style?: 'fill' | 'flat' | 'line' | 'monochrome'
  size?: number | string
}

const props = withDefaults(defineProps<WeatherIconProps>(), {
  style: 'fill',
  size: 64
})

const src = ref('')

watchEffect(async () => {
  if (!props.slug) {
    src.value = ''
    return
  }
  try {
    // relative path to help Vite's static analysis
    const mod = await import(`../../../node_modules/@meteocons/svg/${props.style}/${props.slug}.svg?url`)
    src.value = mod.default
  } catch (err) {
    console.error(`Failed to load weather icon: ${props.slug}`, err)
    src.value = ''
  }
})
</script>

<template>
  <img
    v-if="src"
    v-bind="$attrs"
    :src="src"
    :alt="slug"
    :style="{
      width: typeof size === 'number' ? `${size}px` : size,
      height: typeof size === 'number' ? `${size}px` : size
    }"
  />
  <div
    v-else
    v-bind="$attrs"
    :style="{
      width: typeof size === 'number' ? `${size}px` : size,
      height: typeof size === 'number' ? `${size}px` : size
    }"
  />
</template>
