<script setup lang="ts">
import type {Component} from 'vue'

defineProps<{label: string; value: string; icon: Component; open?: boolean}>()
</script>

<template>
  <button type="button" class="field" :class="{'is-open': open}">
    <span class="field-label">{{ label }}</span>
    <span class="field-value">{{ value }}</span>
    <component :is="icon" class="m-icon field-icon" />
  </button>
</template>

<style scoped>
.field {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-width: 0;
  height: 56px;
  padding: 0 12px 0 16px;
  border: 0;
  border-radius: 14px;
  background: transparent;
  box-shadow: inset 0 0 0 1px var(--md-outline);
  color: var(--md-on-surface);
  font-size: 16px;
  line-height: 24px;
  letter-spacing: 0.5px;
  text-align: left;
  cursor: pointer;
  transition: box-shadow 130ms linear;
}

@media (hover: hover) {
  .field:hover {
    box-shadow: inset 0 0 0 1px var(--md-on-surface);
  }
}

.field:focus-visible,
.field.is-open {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--md-primary);
}

.field-label {
  position: absolute;
  top: -8px;
  left: 12px;
  padding: 0 4px;
  background: var(--md-surface-container-low);
  font-size: 12px;
  line-height: 16px;
  letter-spacing: 0.4px;
  color: var(--md-on-surface-variant);
}

.field:focus-visible .field-label,
.field.is-open .field-label {
  color: var(--md-primary);
}

.field-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-icon {
  color: var(--md-on-surface-variant);
  transition: rotate var(--spring-fast-ms) var(--spring-fast);
}

.field.is-open .field-icon {
  rotate: 180deg;
}
</style>
