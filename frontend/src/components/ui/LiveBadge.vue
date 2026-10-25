<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import RealtimeIcon from '@/components/ui/RealtimeIcon.vue'

const {t} = useI18n()
</script>

<template>
  <span class="live-badge">
    <RealtimeIcon class="size-[9px]" />
    <span>{{ t('live') }}</span>
  </span>
</template>

<style scoped>
.live-badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
  padding: 1px 5px;
  border-radius: 6px;
  background: var(--md-live-container);
  color: var(--md-on-live-container);
  font-size: 11px;
  line-height: 16px;
  font-weight: 700;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}
</style>
