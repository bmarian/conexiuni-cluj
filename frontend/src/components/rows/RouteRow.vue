<script setup lang="ts">
import LineBadge from '@/components/ui/LineBadge.vue'
import {KeyboardArrowRightIcon} from '@/utils/icons.ts'

defineProps<{ name: string, color?: string, title: string, favorite?: boolean, busy?: boolean }>()
defineEmits<{ open: [] }>()
</script>

<template>
  <div
    class="route-row state squish"
    :class="{ 'is-favorite': favorite, 'is-busy': busy }"
    role="button"
    tabindex="0"
    @click="$emit('open')"
    @keydown.enter.prevent="$emit('open')"
  >
    <LineBadge :name="name" :color="color" :height="28" :min-width="40" :corner="6"/>
    <span class="flex-1 min-w-0 truncate text-body-medium">{{ title }}</span>
    <span v-if="busy" class="spinner"></span>
    <KeyboardArrowRightIcon v-else class="m-icon size-[18px] text-outline"/>
  </div>
</template>

<style scoped>
.route-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  color: var(--md-on-surface);
  cursor: pointer;
}

.route-row.is-favorite {
  background: color-mix(in srgb, var(--md-rose) 6%, transparent);
}

.route-row.is-busy {
  opacity: 0.6;
  pointer-events: none;
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--md-primary);
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 800ms linear infinite;
}

@keyframes spin {
  to {
    rotate: 360deg;
  }
}
</style>
