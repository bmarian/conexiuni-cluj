<script setup lang="ts" generic="T extends string">
import {computed, ref, watch} from 'vue'
import PopIcon from '@/components/ui/PopIcon.vue'
import type {Choice} from '@/components/ui/types.ts'

const props = defineProps<{ options: Choice<T>[], modelValue: T, kbd?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: T] }>()

const index = computed(() => Math.max(0, props.options.findIndex((o) => o.value === props.modelValue)))
const moving = ref(0)
watch(index, () => moving.value++)
</script>

<template>
  <div class="choice-row" role="radiogroup" :style="{ '--n': options.length, '--i': index }" :data-kbd-section="kbd" data-kbd-axis="x">
    <span class="pill" :class="moving ? (moving % 2 ? 'stretch-a' : 'stretch-b') : undefined"></span>
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      class="choice squish"
      :class="{ 'is-checked': option.value === modelValue }"
      :aria-checked="option.value === modelValue"
      :data-kbd-item="kbd ? `${kbd}-${option.value}` : undefined"
      :data-kbd-active="option.value === modelValue"
      :disabled="option.disabled"
      @click="emit('update:modelValue', option.value)"
    >
      <PopIcon v-if="option.icon" :icon="option.icon" :on="option.value === modelValue" class="size-[18px]"/>
      <slot name="label" :option="option" :checked="option.value === modelValue">
        <span class="truncate">{{ option.label }}</span>
      </slot>
    </button>
  </div>
</template>

<style scoped>
.choice-row {
  position: relative;
  display: grid;
  grid-template-columns: repeat(var(--n), minmax(0, 1fr));
  height: 48px;
  padding: 4px;
  border-radius: 24px;
  background: var(--md-surface-container-highest);
}

.pill {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 4px;
  width: calc((100% - 8px) / var(--n));
  border-radius: 20px;
  background: var(--md-primary);
  translate: calc(var(--i) * 100%) 0;
  transition: translate var(--spring-slide-ms) var(--spring-slide);
}

.pill.stretch-a {
  animation: stretch-a var(--spring-slide-ms) ease-out;
}

.pill.stretch-b {
  animation: stretch-b var(--spring-slide-ms) ease-out;
}

@keyframes stretch-a {
  30% {
    scale: 1.35 0.9;
  }
}

@keyframes stretch-b {
  30% {
    scale: 1.35 0.9;
  }
}

.choice {
  --squish: 0.9;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 0;
  padding: 0 12px;
  border: 0;
  border-radius: 20px;
  background: transparent;
  color: var(--md-on-surface-variant);
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  letter-spacing: 0.1px;
  cursor: pointer;
  transition: color 200ms linear, scale var(--spring-press-ms) var(--spring-press);
}

.choice.is-checked {
  color: var(--md-on-primary);
}

.choice:disabled {
  opacity: 0.38;
  pointer-events: none;
}
</style>
