<script setup lang="ts">
import {onMounted, ref, watch, type Component} from 'vue'
import {ARROW_BACK_MS, ARROW_MS, ARROW_OPEN_DELAY_MS, arrowKeyframes} from '@/utils/directionArrow.ts'

const props = defineProps<{icon: Component; rest: number; heading: number | null; on: boolean}>()

const el = ref<HTMLElement>()
let played = false

function play(delay = 0) {
  const arrow = el.value
  if (!arrow || props.heading === null || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  played = true
  const from = getComputedStyle(arrow).transform
  arrow.getAnimations().forEach((a) => a.cancel())
  if (props.on) arrow.animate(arrowKeyframes(props.rest, props.heading), {duration: ARROW_MS, delay, fill: 'backwards'})
  else arrow.animate([{transform: from}, {transform: 'none'}], {duration: ARROW_BACK_MS, easing: 'ease-in-out'})
}

const open = () => {
  if (props.on && !played) play(ARROW_OPEN_DELAY_MS)
}
onMounted(open)
watch(() => props.heading !== null, open)
watch(
  () => props.on,
  () => play(),
)
</script>

<template>
  <span ref="el" class="inline-flex shrink-0" :style="on && heading !== null ? {transform: `rotate(${heading - rest}deg)`} : undefined">
    <component :is="icon" class="m-icon size-4" />
  </span>
</template>
