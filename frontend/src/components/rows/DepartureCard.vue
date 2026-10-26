<script setup lang="ts">
import type {VehiclesInStop} from '@/types/tranzy.ts'
import {routeDestination, routeOrigin} from '@/utils/time.ts'
import {KeyboardArrowRightIcon} from '@/utils/icons.ts'
import {modeOf} from '@/utils/modes.ts'
import LineBadge from '@/components/ui/LineBadge.vue'
import LiveBadge from '@/components/ui/LiveBadge.vue'
import TimePill from '@/components/ui/TimePill.vue'

defineProps<{
  shape: VehiclesInStop
  label: (minutes: number) => string
  favorite?: boolean
}>()
const emit = defineEmits<{open: []}>()
</script>

<template>
  <div class="departure-card squish" :class="{'is-favorite': favorite}" role="button" tabindex="0" @click="emit('open')" @keydown.enter.prevent="emit('open')">
    <span class="live-bar" :class="{'is-live': !shape.static_time_approximation}"></span>
    <LineBadge :name="shape.route_short_name" :color="shape.route_color" :height="36" :min-width="44" :corner="12" />
    <div class="flex-1 min-w-0 flex flex-col gap-0.5 py-2.5">
      <div class="flex items-center gap-1.5 min-w-0">
        <LiveBadge v-if="!shape.static_time_approximation" />
        <span class="destination text-title-small">→ {{ routeDestination(shape.route_long_name) }}</span>
      </div>
      <span v-if="routeOrigin(shape.route_long_name)" class="text-body-small text-on-surface-variant truncate">
        {{ routeOrigin(shape.route_long_name) }}
      </span>
      <div class="flex flex-wrap items-center gap-1 pt-1">
        <TimePill
          v-for="(time, i) in shape.next_times"
          :key="i"
          :text="i === 0 && shape.static_time_approximation ? `~ ${label(time.minutes)}` : label(time.minutes)"
          :live="i === 0 && !shape.static_time_approximation"
          :extra="i > 0"
          :due="i === 0 && !shape.static_time_approximation && time.minutes === 0"
          :mode="modeOf(shape.route_type)"
        />
      </div>
    </div>
    <KeyboardArrowRightIcon class="m-icon size-5 text-outline" />
  </div>
</template>

<style scoped>
.departure-card {
  position: relative;
  isolation: isolate;
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 4px 12px;
  padding-right: 8px;
  border-radius: 12px;
  background: var(--md-surface-container);
  color: var(--md-on-surface);
  cursor: pointer;
}

.departure-card::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: currentColor;
  opacity: 0;
  transition: opacity 130ms linear;
  z-index: -1;
}

@media (hover: hover) {
  .departure-card:hover::after {
    opacity: 0.08;
  }
}

.departure-card:active::after {
  opacity: 0.1;
}

.departure-card.is-favorite {
  background: var(--md-surface-container-high);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--md-rose) 45%, transparent);
}

.live-bar {
  align-self: stretch;
  width: 4px;
  margin: 10px 0;
  border-radius: 2px;
  flex-shrink: 0;
}

.live-bar.is-live {
  background: var(--md-live);
}

.destination {
  min-width: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
