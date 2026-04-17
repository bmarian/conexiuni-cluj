<script setup lang="ts">
import {computed, nextTick, ref, watch} from "vue"
import MapComponent from "@/components/MapComponent.vue"

type DrawerState = 'collapsed' | 'half' | 'expanded'

// Snap heights expressed as fractions of the dynamic viewport (dvh).
const SNAP_FRAC: Record<DrawerState, number> = {
  collapsed: 0.22,
  half: 0.60,
  expanded: 0.92,
}
const cycleOrder: DrawerState[] = ['collapsed', 'half', 'expanded']

const drawerState = ref<DrawerState>('half')

// When the user is actively dragging we bypass the snap table and use the raw
// pixel height, with CSS transitions disabled so the drawer sticks to the finger.
const isDragging = ref(false)
const dragHeightPx = ref<number | null>(null)

const drawerStyle = computed(() => {
  if (dragHeightPx.value != null) return { height: `${dragHeightPx.value}px` }
  return { height: `${SNAP_FRAC[drawerState.value] * 100}dvh` }
})

// Let Leaflet know its container resized after the drawer snap animation.
watch([drawerState, isDragging], async ([, dragging]) => {
  if (dragging) return
  await nextTick()
  setTimeout(() => window.dispatchEvent(new Event('resize')), 280)
})

// --- Drag handling ---------------------------------------------------------
// PointerEvents unify mouse + touch. We track the starting pointer Y and the
// starting drawer height; every move updates dragHeightPx. On release we snap
// to the closest of the three states.

let pointerId = -1
let startY = 0
let startHeight = 0
let moved = false

function viewportPx() {
  // 1dvh in pixels — falls back to innerHeight if dvh isn't supported.
  return window.visualViewport?.height ?? window.innerHeight
}

function currentHeightPx(el: HTMLElement) {
  return el.getBoundingClientRect().height
}

function onPointerDown(e: PointerEvent) {
  // Only left button / primary touch.
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
  // Drag up → bigger drawer (dy negative), drag down → smaller.
  const vh = viewportPx()
  const min = SNAP_FRAC.collapsed * vh
  const max = SNAP_FRAC.expanded * vh
  dragHeightPx.value = Math.max(min, Math.min(max, startHeight - dy))
}

function endDrag() {
  if (!isDragging.value) return
  const vh = viewportPx()
  const height = dragHeightPx.value ?? startHeight
  // Snap to the nearest state by fraction of the viewport.
  const frac = height / vh
  let best: DrawerState = 'half'
  let bestDist = Infinity
  for (const s of cycleOrder) {
    const d = Math.abs(SNAP_FRAC[s] - frac)
    if (d < bestDist) { bestDist = d; best = s }
  }
  // If the user barely moved, treat it as a tap and cycle instead.
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
  ;(e.currentTarget as HTMLElement).releasePointerCapture?.(e.pointerId)
  endDrag()
}
</script>

<template>
  <main class="app-shell bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
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
  /* dvh follows the URL bar, so we never overflow under Firefox/Safari chrome. */
  height: 100dvh;
  width: 100vw;
  overflow: hidden;
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
  /* Safe area inset lives INSIDE the drawer so the bottom of the scroll region
     never slides under the Firefox URL bar or iOS home indicator. */
  padding-bottom: env(safe-area-inset-bottom);
}

/* While the finger/mouse is holding the grip, height must follow the pointer
   without easing. */
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
  /* Prevent the browser from claiming vertical swipes for page scroll. */
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

/* Desktop: side-by-side, ignore drawer state entirely. */
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
}
</style>
