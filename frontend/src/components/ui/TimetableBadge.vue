<script setup lang="ts">
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
</script>

<template>
  <span class="timetable-badge">{{ t('timetable') }}</span>
</template>

<style scoped>
.timetable-badge {
  flex-shrink: 0;
  padding: 1px 5px;
  border-radius: 6px;
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
  font-size: 11px;
  line-height: 16px;
  font-weight: 700;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}
</style>
