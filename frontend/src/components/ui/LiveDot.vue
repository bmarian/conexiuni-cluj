<script setup lang="ts">
import {ref, watch} from 'vue'
import {useLiveBeat} from '@/composables/useLiveBeat.ts'
import {pop, ripple} from '@/utils/motion.ts'

const dot = ref<HTMLElement>()

watch(useLiveBeat(), () => {
  if (!dot.value) return
  ripple(dot.value, {pseudoElement: '::after'})
  pop(dot.value, 1.5)
})
</script>

<template>
  <span ref="dot" class="live-dot" aria-hidden="true"></span>
</template>

<style scoped>
.live-dot {
  position: relative;
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--md-live);
}

.live-dot::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: inherit;
  opacity: 0;
  pointer-events: none;
}
</style>
