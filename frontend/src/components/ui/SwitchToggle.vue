<script setup lang="ts">
import {CheckIcon, CloseIcon} from '@/utils/icons.ts'
import PopIcon from '@/components/ui/PopIcon.vue'

defineProps<{checked: boolean; disabled?: boolean}>()
defineEmits<{change: [checked: boolean]}>()
</script>

<template>
  <button
    type="button"
    role="switch"
    class="switch"
    :class="{'is-on': checked}"
    :aria-checked="checked"
    :disabled="disabled"
    @click="$emit('change', !checked)"
  >
    <span class="thumb">
      <PopIcon :icon="checked ? CheckIcon : CloseIcon" :on="checked" class="size-4" />
    </span>
  </button>
</template>

<style scoped>
.switch {
  position: relative;
  flex-shrink: 0;
  width: 52px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 16px;
  background: var(--md-surface-container-highest);
  box-shadow: inset 0 0 0 2px var(--md-outline);
  cursor: pointer;
  transition:
    background-color 150ms linear,
    box-shadow 150ms linear;
}

.thumb {
  position: absolute;
  top: 4px;
  left: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--md-outline);
  color: var(--md-surface-container-highest);
  transition:
    translate var(--spring-fast-ms) var(--spring-fast),
    scale var(--spring-fast-ms) var(--spring-fast),
    background-color 150ms linear,
    color 150ms linear;
}

.switch:active .thumb {
  scale: 1.17;
}

.switch.is-on {
  background: var(--md-primary);
  box-shadow: none;
}

.switch.is-on .thumb {
  translate: 20px 0;
  background: var(--md-on-primary);
  color: var(--md-on-primary-container);
}

:root.dark .switch.is-on .thumb {
  color: var(--md-primary-container);
}

.switch:disabled {
  opacity: 0.38;
  pointer-events: none;
}
</style>
