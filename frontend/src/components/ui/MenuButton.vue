<script setup lang="ts">
import {nextTick, onBeforeUnmount, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {MoreVertIcon} from '@/utils/icons.ts'
import type {MenuAction} from '@/components/ui/types.ts'

defineProps<{ actions: MenuAction[] }>()

const {t} = useI18n()
const button = ref<HTMLElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const open = ref(false)
const at = ref({top: 0, right: 0})

function onOutside(e: Event) {
  const target = e.target as Node
  if (menu.value?.contains(target) || button.value?.contains(target)) return
  close()
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

function close() {
  open.value = false
  document.removeEventListener('pointerdown', onOutside, true)
  document.removeEventListener('keydown', onKey)
}

async function toggle() {
  if (open.value) return close()
  const box = button.value!.getBoundingClientRect()
  at.value = {top: box.bottom + 4, right: window.innerWidth - box.right}
  open.value = true
  document.addEventListener('pointerdown', onOutside, true)
  document.addEventListener('keydown', onKey)
  await nextTick()
  menu.value?.querySelector('button')?.focus({preventScroll: true})
}

function pick(action: MenuAction) {
  close()
  action.onClick()
}

onBeforeUnmount(close)
</script>

<template>
  <button
    ref="button"
    type="button"
    class="icon-btn"
    data-kbd-item="menu"
    :aria-label="t('moreOptions')"
    :title="t('moreOptions')"
    :aria-expanded="open"
    @click="toggle"
  >
    <MoreVertIcon class="m-icon"/>
  </button>
  <Teleport to="body">
    <Transition name="menu">
      <div v-if="open" ref="menu" class="menu" role="menu" :style="{ top: `${at.top}px`, right: `${at.right}px` }">
        <button v-for="action in actions" :key="action.label" type="button" class="menu-item state" role="menuitem" @click="pick(action)">
          <component :is="action.icon" class="m-icon text-on-surface-variant"/>
          <span>{{ action.label }}</span>
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.menu {
  position: fixed;
  z-index: 9000;
  min-width: 200px;
  max-width: 280px;
  padding: 8px 0;
  border-radius: 12px;
  background: var(--md-surface-container);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-2);
  transform-origin: top right;
}

.menu-item {
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

.menu-enter-active {
  transition: scale var(--spring-fast-ms) var(--spring-fast), opacity 120ms linear;
}

.menu-leave-active {
  transition: scale 120ms ease-in, opacity 120ms linear;
}

.menu-enter-from,
.menu-leave-to {
  scale: 0.8;
  opacity: 0;
}
</style>
