<script setup lang="ts">
import {ref, watch} from 'vue'
import {useLiveBeat} from '@/composables/useLiveBeat.ts'
import {reducedMotion} from '@/utils/motion.ts'

const icon = ref<SVGSVGElement>()

watch(useLiveBeat(), () => {
  if (reducedMotion()) return
  ;[...(icon.value?.children ?? [])].forEach((part, i) =>
    part.animate([{opacity: 0.2}, {opacity: 1}], {duration: 210, delay: 120 + i * 95, easing: 'linear', fill: 'backwards'}),
  )
})
</script>

<template>
  <svg ref="icon" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.56" stroke-linecap="round" aria-hidden="true">
    <circle cx="1.56" cy="10.44" r="1.72" fill="currentColor" stroke="none" />
    <path d="M1.56 6.6A3.84 3.84 0 0 1 5.4 10.44" />
    <path d="M1.56 3A7.44 7.44 0 0 1 9 10.44" />
  </svg>
</template>
