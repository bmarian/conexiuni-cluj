<script setup lang="ts">
defineProps<{stats: {value: string; label: string}[]}>()
</script>

<template>
  <div class="admin-stats">
    <div v-for="s in stats" :key="s.label">
      <b>{{ s.value }}</b>
      <span>{{ s.label }}</span>
    </div>
  </div>
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
</style>
