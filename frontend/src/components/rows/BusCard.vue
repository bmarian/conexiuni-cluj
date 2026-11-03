<script setup lang="ts">
import {KeyboardArrowRightIcon} from '@/utils/icons.ts'
import LiveBadge from '@/components/ui/LiveBadge.vue'

defineProps<{live: boolean; destination: string; subtitle?: string; favorite?: boolean}>()
const emit = defineEmits<{open: []}>()
</script>

<template>
  <div class="bus-card squish" :class="{'is-favorite': favorite}" role="button" tabindex="0" @click="emit('open')" @keydown.enter.prevent="emit('open')">
    <span class="live-bar" :class="{'is-live': live}"></span>
    <slot name="badge" />
    <div class="flex-1 min-w-0 flex flex-col gap-0.5 py-2.5">
      <div class="flex items-center gap-1.5 min-w-0">
        <LiveBadge v-if="live" />
        <span class="destination text-title-small">→ {{ destination }}</span>
      </div>
      <span v-if="subtitle" class="text-body-small text-on-surface-variant truncate">{{ subtitle }}</span>
      <slot />
    </div>
    <KeyboardArrowRightIcon class="m-icon size-5 text-outline" />
  </div>
</template>

<style scoped>
.bus-card {
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

.bus-card::after {
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
  .bus-card:hover::after {
    opacity: 0.08;
  }
}

.bus-card:active::after {
  opacity: 0.1;
}

.bus-card.is-favorite {
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
