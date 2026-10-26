<script setup lang="ts">
import PopIcon from '@/components/ui/PopIcon.vue'
import type {ChipAction} from '@/components/ui/types.ts'

defineProps<{actions: ChipAction[]}>()
</script>

<template>
  <div class="action-chips" data-kbd-section="actions" data-kbd-axis="x">
    <button
      v-for="(action, i) in actions"
      :key="i"
      type="button"
      class="chip squish"
      :class="{'is-primary': action.primary, 'is-selected': action.selected}"
      :disabled="action.disabled"
      :data-kbd-item="action.kbd ?? `action-${i}`"
      @click="action.onClick()"
    >
      <PopIcon :icon="action.icon" :on="!!action.selected" class="size-[18px]" :style="action.tint ? {color: action.tint} : undefined" />
      <span>{{ action.label }}</span>
    </button>
  </div>
</template>

<style scoped>
/* Layered so page utilities can override it. */
@layer components {
  .action-chips {
    display: flex;
    gap: 8px;
    padding: 0 16px;
    overflow-x: auto;
    scrollbar-width: none;
  }
}

.action-chips::-webkit-scrollbar {
  display: none;
}

.chip {
  --squish: 0.92;
  position: relative;
  isolation: isolate;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  height: 36px;
  margin-block: 6px;
  padding: 0 16px 0 12px;
  border: 0;
  border-radius: 18px;
  background: transparent;
  color: var(--md-on-surface);
  box-shadow: inset 0 0 0 1px var(--md-outline-variant);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.1px;
  white-space: nowrap;
  cursor: pointer;
  transition:
    border-radius var(--spring-fast-ms) var(--spring-fast),
    scale var(--spring-press-ms) var(--spring-press),
    background-color 175ms linear,
    color 175ms linear;
}

.chip::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: currentColor;
  opacity: 0;
  transition: opacity 130ms linear;
  z-index: -1;
}

@media (hover: hover) {
  .chip:hover::after {
    opacity: 0.08;
  }
}

.chip:active {
  border-radius: 8px;
}

.chip:active::after {
  opacity: 0.1;
}

.chip.is-primary {
  background: var(--md-primary);
  color: var(--md-on-primary);
  box-shadow: none;
}

.chip.is-selected {
  border-radius: 12px;
  background: var(--md-secondary-container);
  color: var(--md-on-secondary-container);
  box-shadow: none;
}

.chip.is-selected:active {
  border-radius: 8px;
}

.chip:disabled {
  opacity: 0.38;
  pointer-events: none;
}
</style>
