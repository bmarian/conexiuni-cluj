<script setup lang="ts">
import type {Component} from 'vue'

defineProps<{icon?: Component; title: string; description?: string; as?: string | Component}>()
</script>

<template>
  <component :is="as ?? 'button'" class="list-item state" :type="(as ?? 'button') === 'button' ? 'button' : undefined">
    <slot name="leading">
      <component :is="icon" v-if="icon" class="m-icon text-on-surface-variant" />
    </slot>
    <span class="flex-1 min-w-0 flex flex-col text-left">
      <span class="flex items-center gap-1.5">
        <span class="text-body-large text-on-surface">{{ title }}</span>
        <slot name="badge" />
      </span>
      <span v-if="description" class="text-body-medium text-on-surface-variant">{{ description }}</span>
    </span>
    <slot name="trailing" />
  </component>
</template>

<style scoped>
.list-item {
  display: flex;
  align-items: center;
  gap: 16px;
  width: calc(100% - 8px);
  margin: 0 4px;
  min-height: 56px;
  padding: 8px 24px 8px 16px;
  border: 0;
  border-radius: 16px;
  background: transparent;
  color: var(--md-on-surface);
  text-decoration: none;
  cursor: pointer;
}
</style>
