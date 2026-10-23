<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {displayColor} from '@/utils/lineColor.ts'

const {name, color, height = 28, minWidth = 40, corner = 8} = defineProps<{
  name: string
  color?: string
  height?: number
  minWidth?: number
  corner?: number
}>()

const shown = computed(() => displayColor(name, color ?? '') || 'var(--md-outline)')

// The colour it had before a recolour of the same line, which the new one fills over.
const previous = ref<string>()
const run = ref(0)
watch([() => name, shown], ([line, now], [before, then]) => {
  if (line !== before) {
    previous.value = undefined
    run.value = 0
  } else if (now !== then) {
    previous.value = then
    run.value++
  }
})
</script>

<template>
  <span
    :key="run"
    class="line-badge"
    :class="{ 'is-recolored': run }"
    :style="{
      backgroundColor: previous ?? shown,
      minHeight: `${height}px`,
      minWidth: `${minWidth}px`,
      borderRadius: `${corner}px`,
      fontSize: `${height * 0.45}px`,
    }"
  >
    <span v-if="run" class="fill" :style="{ backgroundColor: shown }"></span>
    <span class="name">{{ name }}</span>
  </span>
</template>

<style scoped>
.line-badge {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  padding: 0 6px;
  color: #fff;
  font-weight: 900;
  line-height: 1;
  white-space: nowrap;
}

.name {
  position: relative;
}

.fill {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  animation: fill-in var(--spring-fast-ms) var(--spring-fast) both;
}

.is-recolored {
  animation: pop var(--spring-pop-ms) var(--spring-pop);
}

@keyframes fill-in {
  from {
    clip-path: circle(0 at 50% 50%);
  }
  to {
    clip-path: circle(75% at 50% 50%);
  }
}

@keyframes pop {
  from {
    scale: 0.8;
  }
}

@media (prefers-reduced-motion: reduce) {
  .fill,
  .is-recolored {
    animation: none;
  }
}
</style>
