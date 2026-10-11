<script setup lang="ts">
import {nextTick, onMounted, onUnmounted, ref} from 'vue'
import {useKbdEscape} from '@/composables/useKeyboardNav.ts'

export interface BusMenuItem {
  key: string
  label: string
  icon: 'bell' | 'bell-off' | 'alarm' | 'wake'
  action: () => void
}

const props = defineProps<{ x: number, y: number, items: BusMenuItem[] }>()
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
      <button v-for="item in items" :key="item.key" type="button" role="menuitem" class="bus-menu-item" @click="run(item)">
        <svg v-if="item.icon === 'bell'" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 17h5l-1.4-1.4A2 2 0 0 1 18 14.2V11a6 6 0 1 0-12 0v3.2a2 2 0 0 1-.6 1.4L4 17h5m6 0a3 3 0 1 1-6 0m6 0H9"/>
        </svg>
        <svg v-else-if="item.icon === 'bell-off'" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M13.7 21a2 2 0 0 1-3.4 0M18.6 13A17.9 17.9 0 0 1 18 8M6.3 6.3A5.9 5.9 0 0 0 6 8c0 7-3 9-3 9h14M18 8a6 6 0 0 0-9.3-5M2 2l20 20"/>
        </svg>
        <svg v-else-if="item.icon === 'alarm'" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="13" r="8"/><path stroke-linecap="round" d="M12 9v4l2 2M5 3L2 6m17-3l3 3M12 2v1"/>
        </svg>
        <svg v-else class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="13" r="8"/><path stroke-linecap="round" d="M12 9v4l2 2M5 3L2 6m17-3l3 3"/>
        </svg>
        <span>{{ item.label }}</span>
      </button>
    </div>
  </Teleport>
</template>

<style scoped>
.bus-menu {
  position: fixed;
  z-index: 10001;
  min-width: 13rem;
  padding: 0.375rem;
  border-radius: 0.875rem;
  background: var(--color-white, #fff);
  color: var(--color-slate-800, #1e293b);
  border: 1px solid var(--color-slate-200, #e2e8f0);
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.18);
}

.bus-menu-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.6rem 0.75rem;
  border-radius: 0.6rem;
  font-size: 0.875rem;
  font-weight: 500;
  text-align: left;
}

.bus-menu-item:hover,
.bus-menu-item:focus-visible {
  background: var(--color-slate-100, #f1f5f9);
  outline: none;
}

:root.dark .bus-menu {
  background: var(--color-slate-800, #1e293b);
  color: var(--color-slate-100, #f1f5f9);
  border-color: var(--color-slate-700, #334155);
}

:root.dark .bus-menu-item:hover,
:root.dark .bus-menu-item:focus-visible {
  background: var(--color-slate-700, #334155);
}
</style>
