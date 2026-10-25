<script setup lang="ts">
import {computed, onBeforeUnmount, ref} from 'vue'
import type {CitySummary} from '@/components/admin/types.ts'
import {CheckIcon, ExpandMoreIcon} from '@/utils/icons.ts'

const props = defineProps<{ cities: CitySummary[], city: string }>()
const emit = defineEmits<{ pick: [slug: string] }>()

const root = ref<HTMLElement | null>(null)
const open = ref(false)
const name = computed(() => props.cities.find((c) => c.slug === props.city)?.name ?? props.city)

function onOutside(e: Event) {
  if (!root.value?.contains(e.target as Node)) close()
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

function close() {
  open.value = false
  document.removeEventListener('pointerdown', onOutside, true)
  document.removeEventListener('keydown', onKey)
}

function toggle() {
  if (open.value) return close()
  open.value = true
  document.addEventListener('pointerdown', onOutside, true)
  document.addEventListener('keydown', onKey)
}

function pick(slug: string) {
  close()
  emit('pick', slug)
}

onBeforeUnmount(close)
</script>

<template>
  <div ref="root" class="city-menu">
    <button type="button" class="trigger state" aria-haspopup="menu" :aria-expanded="open" @click="toggle">
      <h2 class="text-title-large truncate">{{ name }}</h2>
      <ExpandMoreIcon class="m-icon chevron" :class="{ 'is-open': open }"/>
    </button>
    <Transition name="menu">
      <div v-if="open" class="menu" role="menu">
        <button
          v-for="c in cities"
          :key="c.slug"
          type="button"
          role="menuitemradio"
          class="item state"
          :aria-checked="c.slug === city"
          @click="pick(c.slug)"
        >
          <span class="flex-1 truncate">{{ c.name }}</span>
          <CheckIcon v-if="c.slug === city" class="m-icon text-primary"/>
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.city-menu {
  position: relative;
  min-width: 0;
}

.trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  margin-left: -8px;
  padding: 2px 4px 2px 8px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  color: var(--md-on-surface);
  cursor: pointer;
}

.chevron {
  color: var(--md-on-surface-variant);
  transition: rotate 200ms ease;
}

.chevron.is-open {
  rotate: 180deg;
}

.menu {
  position: absolute;
  top: calc(100% + 4px);
  left: -8px;
  z-index: 10;
  min-width: 220px;
  max-height: min(420px, 60vh);
  overflow-y: auto;
  padding: 8px 0;
  border-radius: 12px;
  background: var(--md-surface-container);
  color: var(--md-on-surface);
  box-shadow: var(--md-elevation-2);
  transform-origin: top left;
}

.item {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  height: 48px;
  padding: 0 12px 0 16px;
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
