<script setup lang="ts">
import {computed, nextTick, ref, watch} from "vue"
import {useI18n} from "vue-i18n"
import MapComponent from "@/components/MapComponent.vue"
import {useOnline} from "@/composables/useOnline"

const {t} = useI18n()
const {isOnline} = useOnline()

type DrawerState = 'collapsed' | 'half' | 'expanded'

const SNAP_FRAC: Record<DrawerState, number> = {
  collapsed: 0.22,
  half: 0.60,
  expanded: 0.92,
}
const cycleOrder: DrawerState[] = ['collapsed', 'half', 'expanded']

const drawerState = ref<DrawerState>('half')

const isDragging = ref(false)
const dragHeightPx = ref<number | null>(null)

const drawerStyle = computed(() => {
  if (dragHeightPx.value != null) return { height: `${dragHeightPx.value}px` }
  return { height: `${SNAP_FRAC[drawerState.value] * 100}dvh` }
})

watch([drawerState, isDragging], async ([, dragging]) => {
  if (dragging) return
  await nextTick()
  setTimeout(() => window.dispatchEvent(new Event('resize')), 280)
})

let pointerId = -1
let startY = 0
let startHeight = 0
let moved = false

function viewportPx() {
  return window.visualViewport?.height ?? window.innerHeight
}

function currentHeightPx(el: HTMLElement) {
  return el.getBoundingClientRect().height
}

function onPointerDown(e: PointerEvent) {
  if (e.button !== 0 && e.pointerType === 'mouse') return
  const el = (e.currentTarget as HTMLElement).closest('.app-drawer') as HTMLElement | null
  if (!el) return
  pointerId = e.pointerId
  startY = e.clientY
  startHeight = currentHeightPx(el)
  moved = false
  isDragging.value = true
  dragHeightPx.value = startHeight
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  if (!isDragging.value || e.pointerId !== pointerId) return
  const dy = e.clientY - startY
  if (Math.abs(dy) > 3) moved = true
  const vh = viewportPx()
  const min = SNAP_FRAC.collapsed * vh
  const max = SNAP_FRAC.expanded * vh
  dragHeightPx.value = Math.max(min, Math.min(max, startHeight - dy))
}

function endDrag() {
  if (!isDragging.value) return
  const vh = viewportPx()
  const height = dragHeightPx.value ?? startHeight
  const frac = height / vh
  let best: DrawerState = 'half'
  let bestDist = Infinity
  for (const s of cycleOrder) {
    const d = Math.abs(SNAP_FRAC[s] - frac)
    if (d < bestDist) { bestDist = d; best = s }
  }
  if (!moved) {
    const i = cycleOrder.indexOf(drawerState.value)
    best = cycleOrder[(i + 1) % cycleOrder.length]!
  }
  drawerState.value = best
  isDragging.value = false
  dragHeightPx.value = null
  pointerId = -1
}

function onPointerUp(e: PointerEvent) {
  if (e.pointerId !== pointerId) return
  const el = e.currentTarget as HTMLElement
  if (el.hasPointerCapture?.(e.pointerId)) {
    try { el.releasePointerCapture(e.pointerId) } catch { /* noop */ }
  }
  endDrag()
}
</script>

<template>
  <main class="app-shell bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
    <div v-if="!isOnline" class="offline-pill" role="status" aria-live="polite" :title="t('offlineBanner')">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M2 2l20 20"/>
        <path d="M8.5 16.5a5 5 0 017 0"/>
        <path d="M2 8.82a15 15 0 014.17-2.65"/>
        <path d="M10.66 5c4.01-.36 8.14.9 11.34 3.76"/>
        <path d="M16.85 11.25a10 10 0 012.22 1.68"/>
        <path d="M5 13a10 10 0 015.17-2.69"/>
        <line x1="12" y1="20" x2="12.01" y2="20"/>
      </svg>
      <span>{{ t('offlineShort') }}</span>
    </div>
    <MapComponent class="app-map" />

    <aside
      class="app-drawer bg-slate-100 dark:bg-slate-900 shadow-xl/30"
      :class="{ 'is-dragging': isDragging }"
      :style="drawerStyle"
    >
      <div
        class="drawer-handle lg:hidden"
        role="button"
        tabindex="0"
        aria-label="Drag or tap to resize panel"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
      >
        <span class="drawer-grip"></span>
      </div>
      <div class="drawer-scroll">
        <RouterView />
      </div>
    </aside>
  </main>
</template>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100dvh;
  width: 100vw;
  overflow: hidden;
}

.offline-pill {
  position: fixed;
  top: calc(0.75rem + env(safe-area-inset-top));
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.65rem;
  background: rgba(180, 83, 9, 0.95);
  color: #fff;
  font-size: 0.75rem;
  font-weight: 600;
  border-radius: 9999px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
  pointer-events: none;
  backdrop-filter: blur(4px);
}
.offline-pill svg {
  width: 0.85rem;
  height: 0.85rem;
  flex-shrink: 0;
}

.app-map {
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
}

.app-drawer {
  position: relative;
  flex-shrink: 0;
  width: 100%;
  border-top-left-radius: 1.25rem;
  border-top-right-radius: 1.25rem;
  display: flex;
  flex-direction: column;
  transition: height 250ms cubic-bezier(0.32, 0.72, 0, 1);
  padding-bottom: env(safe-area-inset-bottom);
}

.app-drawer.is-dragging {
  transition: none;
}

.drawer-handle {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-shrink: 0;
  height: 1.75rem;
  width: 100%;
  background: transparent;
  border: 0;
  cursor: grab;
  touch-action: none;
  user-select: none;
}
.drawer-handle:active { cursor: grabbing; }

.drawer-grip {
  width: 2.5rem;
  height: 0.3125rem;
  border-radius: 9999px;
  background: #cbd5e1;
}
@media (prefers-color-scheme: dark) {
  .drawer-grip { background: #475569; }
}

.drawer-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

@media (max-width: 1023px) and (orientation: landscape) {
  .app-drawer { display: none; }
}

@media (min-width: 1024px) {
  .app-shell {
    flex-direction: row;
    height: 100dvh;
  }
  .app-map {
    width: 70vw;
    height: 100dvh;
  }
  .app-drawer {
    width: 30vw;
    height: 100dvh !important;
    border-radius: 0;
    padding-bottom: 0;
    transition: none;
  }
  .drawer-handle {
    display: none;
  }
}
</style>
