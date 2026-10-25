<script setup lang="ts">
import {WarningAmberIcon} from '@/utils/icons.ts'

defineProps<{ stats: {value: string, label: string}[], problems: string[] }>()
</script>

<template>
  <div class="admin-stats">
    <div v-for="s in stats" :key="s.label">
      <b>{{ s.value }}</b>
      <span>{{ s.label }}</span>
    </div>
  </div>
  <ul v-if="problems.length" class="admin-problems">
    <li v-for="p in problems" :key="p">
      <WarningAmberIcon class="m-icon"/>
      <span>{{ p }}</span>
    </li>
  </ul>
</template>

<style scoped>
.admin-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin: 0 0 12px;
}

.admin-stats > div {
  min-width: 0;
  padding: 12px 14px;
  border-radius: 16px;
  background: var(--md-surface-container);
}

.admin-stats b {
  display: block;
  font-size: 28px;
  line-height: 36px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-stats span {
  display: block;
  font-size: 12px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
}

.admin-problems {
  margin: 0 0 12px;
  padding: 8px 0;
  border-radius: 16px;
  background: var(--md-error-container);
  color: var(--md-on-error-container);
  list-style: none;
}

.admin-problems li {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 16px;
  font-size: 14px;
  line-height: 20px;
  letter-spacing: 0.25px;
  overflow-wrap: anywhere;
}

.admin-problems .m-icon {
  width: 20px;
  height: 20px;
}
</style>
