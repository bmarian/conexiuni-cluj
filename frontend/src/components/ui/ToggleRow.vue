<script setup lang="ts">
import type {Component} from 'vue'
import {useI18n} from 'vue-i18n'
import SwitchToggle from '@/components/ui/SwitchToggle.vue'

defineProps<{
  icon: Component
  title: string
  description?: string
  checked: boolean
  beta?: boolean
  disabled?: boolean
}>()
const emit = defineEmits<{ change: [checked: boolean] }>()

const {t} = useI18n()
</script>

<template>
  <div class="toggle-row state" :class="{ 'is-disabled': disabled }" role="presentation" @click="!disabled && emit('change', !checked)">
    <component :is="icon" class="m-icon text-on-surface-variant"/>
    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-1.5">
        <span class="text-body-large text-on-surface">{{ title }}</span>
        <span v-if="beta" class="beta">{{ t('beta') }}</span>
      </div>
      <p v-if="description" class="text-body-medium text-on-surface-variant">{{ description }}</p>
      <slot/>
    </div>
    <SwitchToggle :checked="checked" :disabled="disabled" @click.stop @change="emit('change', $event)"/>
  </div>
</template>

<style scoped>
.toggle-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 0 4px;
  padding: 8px 24px 8px 16px;
  border-radius: 16px;
  cursor: pointer;
  color: var(--md-on-surface);
}

.toggle-row.is-disabled {
  cursor: default;
}

.beta {
  padding: 1px 6px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--md-amber) 18%, transparent);
  color: #b45309;
  font-size: 11px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  text-transform: uppercase;
}
</style>
