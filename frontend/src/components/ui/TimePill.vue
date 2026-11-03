<script setup lang="ts">
import {ref, watch} from 'vue'
import RealtimeIcon from '@/components/ui/RealtimeIcon.vue'
import Ticker from '@/components/ui/Ticker.vue'
import {MODE_ICON} from '@/utils/modeIcons.ts'
import type {Mode} from '@/utils/modes.ts'
import {delayed, EMPHASIZED_DECELERATE, paced, pop, reducedMotion, squash} from '@/utils/motion.ts'

const props = defineProps<{text: string; live?: boolean; extra?: boolean; due?: boolean; mode?: Mode}>()

const pill = ref<HTMLElement>()
const vehicle = ref<SVGSVGElement>()

watch(
  () => [props.due, props.live] as const,
  ([due], [wasDue, wasLive]) => {
    if (!due || wasDue || !wasLive || reducedMotion()) return
    vehicle.value?.animate([{scale: 0.15, opacity: 0}, {scale: 1, opacity: 1}], {duration: paced(540), delay: delayed(70), easing: EMPHASIZED_DECELERATE, fill: 'backwards'})
    squash(vehicle.value, delayed(610))
    pop(pill.value, 0.86, delayed(540))
  },
  {flush: 'post'},
)
</script>

<template>
  <span ref="pill" class="time-pill" :class="{'is-live': live, 'is-extra': extra && !live, 'is-due': due}">
    <span v-if="live" class="glyph">
      <Transition name="shrink">
        <RealtimeIcon v-if="!due" />
      </Transition>
      <svg v-if="due" ref="vehicle" class="fill-current" viewBox="0 0 24 24" aria-hidden="true" v-html="MODE_ICON[mode ?? 'bus']"></svg>
    </span>
    <Ticker :text="text" />
  </span>
</template>

<style scoped>
.time-pill {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 8px;
  border-radius: 8px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  font-size: 13px;
  line-height: 18px;
  font-weight: 700;
}

.time-pill.is-live {
  background: var(--md-live-container);
  color: var(--md-on-live-container);
}

.time-pill.is-extra {
  background: transparent;
  font-weight: 500;
}

.glyph {
  display: inline-grid;
  width: 10px;
  height: 10px;
}

.glyph > * {
  grid-area: 1 / 1;
  width: 100%;
  height: 100%;
}

.shrink-leave-active {
  transition:
    scale 140ms ease-in,
    opacity 140ms ease-in;
}

.shrink-leave-to {
  scale: 0;
  opacity: 0;
}

.time-pill.is-due {
  background: var(--md-live);
  color: var(--md-live-container);
  transition:
    background-color 190ms linear calc(485ms * var(--pace-delay)),
    color 190ms linear calc(485ms * var(--pace-delay));
}

@media (prefers-reduced-motion: reduce) {
  .time-pill {
    transition: none !important;
  }
}
</style>
