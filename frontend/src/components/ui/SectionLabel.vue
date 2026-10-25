<script setup lang="ts">
import type {Component} from 'vue'

defineProps<{text: string; icon?: Component}>()
</script>

<template>
  <div class="section-label">
    <component :is="icon" v-if="icon" class="m-icon size-4" />
    <h2 class="text-title-small truncate">{{ text }}</h2>
    <template v-if="$slots.trailing">
      <span class="flex-1"></span>
      <slot name="trailing" />
    </template>
  </div>
</template>

<style scoped>
.section-label {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  color: var(--md-on-surface-variant);
}
</style>
