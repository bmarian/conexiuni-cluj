<script setup lang="ts">
import type {Component} from 'vue'
import RowIcon from '@/components/ui/RowIcon.vue'

defineProps<{ icon: Component, tone: 'primary' | 'tertiary', title?: string }>()
</script>

<template>
  <div class="admin-scope">
    <RowIcon :icon="icon" :tone="tone" :size="40"/>
    <div class="min-w-0 flex-1">
      <slot name="title">
        <h2 class="text-title-large truncate">{{ title }}</h2>
      </slot>
      <div class="text-body-small text-on-surface-variant flex items-center gap-1.5">
        <slot/>
      </div>
    </div>
  </div>
</template>

<style scoped>
.admin-scope {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 4px 12px;
}
</style>
