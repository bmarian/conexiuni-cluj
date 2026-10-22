<script setup lang="ts">
import {nextTick, onMounted, onUnmounted, ref} from 'vue'
import {useKbdEscape} from '@/composables/useKeyboardNav.ts'
import {AlarmAddIcon, BedtimeIcon, NotificationsOffIcon} from '@/utils/icons.ts'

export interface BusMenuItem {
  key: string
  label: string
  icon: 'bell-off' | 'alarm' | 'wake'
  action: () => void
}

const props = defineProps<{ x: number, y: number, items: BusMenuItem[] }>()

const ICONS = {'bell-off': NotificationsOffIcon, alarm: AlarmAddIcon, wake: BedtimeIcon}
const emit = defineEmits<{ close: [] }>()

const el = ref<HTMLElement | null>(null)
const pos = ref({left: props.x, top: props.y})

function onDocument(e: Event) {
  if (el.value && !el.value.contains(e.target as Node)) emit('close')
}

onMounted(async () => {
  await nextTick()
  const box = el.value?.getBoundingClientRect()
  if (box) {
    pos.value = {
      left: Math.max(8, Math.min(props.x, window.innerWidth - box.width - 8)),
      top: Math.max(8, Math.min(props.y, window.innerHeight - box.height - 8)),
    }
  }
  el.value?.querySelector('button')?.focus()
  document.addEventListener('pointerdown', onDocument, true)
  window.addEventListener('scroll', onScroll, true)
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', onDocument, true)
  window.removeEventListener('scroll', onScroll, true)
})

function onScroll() {
  emit('close')
}

useKbdEscape(() => true, () => emit('close'))

function run(item: BusMenuItem) {
  emit('close')
  item.action()
}
</script>

<template>
  <Teleport to="body">
    <div ref="el" class="bus-menu" role="menu" :style="{ left: `${pos.left}px`, top: `${pos.top}px` }">
      <button v-for="item in items" :key="item.key" type="button" role="menuitem" class="bus-menu-item state" @click="run(item)">
        <component :is="ICONS[item.icon]" class="m-icon text-on-surface-variant"/>
        <span>{{ item.label }}</span>
      </button>
    </div>
  </Teleport>
</template>

<style scoped>
.bus-menu {
  position: fixed;
  z-index: 10001;
  min-width: 200px;
  padding: 8px 0;
  border-radius: 12px;
  background: var(--md-surface-container);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-2);
  animation: menu-in var(--spring-fast-ms) var(--spring-fast);
  transform-origin: top left;
}

@keyframes menu-in {
  from {
    scale: 0.8;
    opacity: 0;
  }
}

.bus-menu-item {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  height: 48px;
  padding: 0 16px 0 12px;
  border: 0;
  background: transparent;
  color: inherit;
  font-size: 14px;
  line-height: 20px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.bus-menu-item:focus-visible {
  outline: none;
  background: color-mix(in srgb, var(--md-on-surface) 10%, transparent);
}
</style>
