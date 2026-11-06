<script setup lang="ts">
import type {Component} from 'vue'

const {size = 36, tone = 'neutral'} = defineProps<{
  icon: Component
  emoji?: string
  size?: number
  tone?: 'neutral' | 'primary' | 'secondary' | 'tertiary'
}>()
</script>

<template>
  <span class="row-icon" :class="`is-${tone}`" :style="{width: `${size}px`, height: `${size}px`}">
    <span v-if="emoji" :style="{fontSize: `${size / 2}px`, lineHeight: 1}">{{ emoji }}</span>
    <component :is="icon" v-else class="m-icon" :style="{width: `${size / 2}px`, height: `${size / 2}px`}" />
  </span>
</template>

<style scoped>
.row-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 50%;
}

.is-neutral {
  background: var(--md-surface-container-highest);
  color: var(--md-on-surface-variant);
}

.is-primary {
  background: var(--md-primary-container);
  color: var(--md-on-primary-container);
}

.is-secondary {
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
}

.is-tertiary {
  background: var(--md-tertiary-container);
  color: var(--md-on-tertiary-container);
}
</style>
