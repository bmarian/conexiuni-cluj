<script setup lang="ts">
import LineBadge from '@/components/ui/LineBadge.vue'
import {CheckIcon} from '@/utils/icons.ts'

defineProps<{name: string; color: string; title: string; checked: boolean}>()
defineEmits<{toggle: []}>()
</script>

<template>
  <button type="button" class="pick-row state" role="checkbox" :aria-checked="checked" @click="$emit('toggle')">
    <LineBadge :name="name" :color="color" plain :height="32" :min-width="40" :corner="10" />
    <span class="flex-1 min-w-0 text-body-medium font-medium truncate">{{ title }}</span>
    <span class="box" :class="{'is-checked': checked}">
      <CheckIcon v-if="checked" class="m-icon size-4" />
    </span>
  </button>
</template>

<style scoped>
.pick-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 60px;
  padding: 0 16px;
  color: var(--md-on-surface);
  text-align: left;
  cursor: pointer;
}

.box {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  margin: 0 3px;
  border: 2px solid var(--md-on-surface-variant);
  border-radius: 2px;
}

.box.is-checked {
  border-color: var(--md-primary);
  background: var(--md-primary);
  color: var(--md-on-primary);
}
</style>
