<script setup lang="ts">
import RealtimeIcon from '@/components/ui/RealtimeIcon.vue'
import Ticker from '@/components/ui/Ticker.vue'

defineProps<{ text: string, live?: boolean, extra?: boolean }>()
</script>

<template>
  <span class="time-pill" :class="{ 'is-live': live, 'is-extra': extra && !live }">
    <RealtimeIcon v-if="live" class="size-2.5"/>
    <Ticker :text="text"/>
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
</style>
