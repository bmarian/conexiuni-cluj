<script setup lang="ts" generic="T extends string">
import {computed, nextTick, ref} from 'vue'
import FieldButton from '@/components/ui/FieldButton.vue'
import {usePopup} from '@/composables/usePopup.ts'
import {useKbdLayer} from '@/composables/useKeyboardNav.ts'
import {ExpandMoreIcon} from '@/utils/icons.ts'
import type {Choice} from '@/components/ui/types.ts'

const GAP = 4
const EDGE = 8

const props = defineProps<{label: string; options: Choice<T>[]; modelValue: T; kbd: string; hideLabel?: boolean}>()
const emit = defineEmits<{'update:modelValue': [value: T]}>()

const field = ref<HTMLElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const at = ref({top: 0, left: 0, width: 0, maxHeight: 0, up: false})
const {open, show, close} = usePopup(() => [menu.value, field.value])
useKbdLayer(open, {el: () => menu.value, close})

const current = computed(() => props.options.find((o) => o.value === props.modelValue)?.label ?? '')

async function toggle() {
  if (open.value) return close()
  const box = field.value!.getBoundingClientRect()
  const below = window.innerHeight - box.bottom - GAP - EDGE
  at.value = {top: box.bottom + GAP, left: box.left, width: box.width, maxHeight: below, up: false}
  show()
  await nextTick()
  if (!menu.value) return
  const left = Math.min(box.left, window.innerWidth - EDGE - menu.value.offsetWidth)
  const height = menu.value.scrollHeight
  const above = box.top - GAP - EDGE
  const up = height > below && above > below
  at.value = {...at.value, left, ...(up && {top: box.top - GAP - Math.min(height, above), maxHeight: above, up})}
}

function pick(value: T) {
  close()
  emit('update:modelValue', value)
}
</script>

<template>
  <div ref="field" class="select-field">
    <FieldButton
      :label="hideLabel ? undefined : label"
      :aria-label="label"
      :value="current"
      :icon="ExpandMoreIcon"
      :open="open"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :data-kbd-item="kbd"
      @click="toggle"
    />
    <Teleport to="body">
      <Transition name="menu">
        <div
          v-if="open"
          ref="menu"
          class="menu popup-menu"
          role="listbox"
          :aria-label="label"
          :class="{'is-up': at.up}"
          :style="{top: `${at.top}px`, left: `${at.left}px`, minWidth: `${at.width}px`, maxHeight: `${at.maxHeight}px`}"
        >
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            class="popup-item state"
            role="option"
            :aria-selected="option.value === modelValue"
            :data-kbd-item="`${kbd}-${option.value}`"
            :data-kbd-entry="option.value === modelValue || undefined"
            :disabled="option.disabled"
            @click="pick(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.select-field {
  min-width: 0;
}

.menu {
  max-width: calc(100vw - 16px);
  overflow-y: auto;
  transform-origin: top center;
}

.menu.is-up {
  transform-origin: bottom center;
}

.popup-item {
  padding: 0 12px;
  white-space: nowrap;
}
</style>
