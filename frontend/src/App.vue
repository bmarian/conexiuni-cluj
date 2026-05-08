<script setup lang="ts">
import {computed, nextTick, onMounted, onUnmounted, ref, watch} from "vue"
import {useI18n} from "vue-i18n"
import MapComponent from "@/components/MapComponent.vue"
import SettingsButton from "@/components/SettingsButton.vue"
import WeatherButton from "@/components/WeatherButton.vue"
import NewsButton from "@/components/NewsButton.vue"
import OfflinePill from "@/components/OfflinePill.vue"
import GreenFridayBanner from "@/components/GreenFridayBanner.vue"
import ArcadeToast from "@/components/ArcadeToast.vue"
import ArcadeTransition from "@/components/ArcadeTransition.vue"
import {useMapStore} from "@/stores/map.ts"
import {useSettingsStore} from "@/stores/settings.ts"

const {t} = useI18n()
const mapStore = useMapStore()
const appSettings = useSettingsStore()

type DrawerState = 'minimized' | 'collapsed' | 'half' | 'expanded' | 'fullscreen'

const MINIMIZED_PX = 44

const SNAP_FRAC: Record<DrawerState, number> = {
  minimized: 0,
  collapsed: 0.22,
  half: 0.60,
  expanded: 0.92,
  fullscreen: 1.0,
}

const drawerEl = ref<HTMLElement | null>(null)
const drawerState = ref<DrawerState>('half')
const isLandscapeDrawerOpen = ref(false)
const isDragging = ref(false)

const attributionHtml = '<a href="https://leafletjs.com" title="A JavaScript library for interactive maps" target="_blank" rel="noopener"><svg aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="12" height="8" viewBox="0 0 12 8" class="leaflet-attribution-flag"><path fill="#4C7BE1" d="M0 0h12v4H0z"></path><path fill="#FFD500" d="M0 4h12v3H0z"></path><path fill="#E0BC00" d="M0 7h12v1H0z"></path></svg> Leaflet</a> | &copy; <a href="https://www.openstreetmap.org/copyright" target="_blank">OpenStreetMap</a>, &copy; <a href="https://carto.com/attributions" target="_blank">CARTO</a> | &copy; <a href="https://tranzy.ai/" target="_blank" rel="noopener">tranzy.ai</a>, &copy; <a href="https://ctpcj.ro" target="_blank" rel="noopener">CTP Cluj-Napoca</a>'

// Landscape and desktop let CSS handle the transform, so drawerStyle must not set one there.
const isPortraitMobile = ref(false)
let mqlLandscape: MediaQueryList | null = null
let mqlDesktop: MediaQueryList | null = null

const updatePortraitMobile = () => {
  isPortraitMobile.value = !(mqlLandscape?.matches || mqlDesktop?.matches)
}

onMounted(() => {
  mqlLandscape = window.matchMedia('(max-width: 1023px) and (orientation: landscape)')
  mqlDesktop = window.matchMedia('(min-width: 1024px)')
  mqlLandscape.addEventListener('change', updatePortraitMobile)
  mqlDesktop.addEventListener('change', updatePortraitMobile)
  updatePortraitMobile()
})

onUnmounted(() => {
  mqlLandscape?.removeEventListener('change', updatePortraitMobile)
  mqlDesktop?.removeEventListener('change', updatePortraitMobile)
})

const drawerStyle = computed(() => {
  if (!isPortraitMobile.value) return {}
  const state = drawerState.value
  if (state === 'fullscreen') return {transform: 'translateY(0px)', '--drawer-visible-h': '100dvh'}
  if (state === 'minimized') return {
    transform: `translateY(calc(100dvh - ${MINIMIZED_PX}px))`,
    '--drawer-visible-h': `${MINIMIZED_PX}px`,
  }
  const hiddenFrac = 1 - SNAP_FRAC[state]
  return {
    transform: `translateY(${hiddenFrac * 100}dvh)`,
    '--drawer-visible-h': `${SNAP_FRAC[state] * 100}dvh`,
  }
})

watch([drawerState, isDragging], async ([, dragging]) => {
  if (dragging) return
  await nextTick()
  setTimeout(() => window.dispatchEvent(new Event('resize')), 280)
})

watch([drawerState, isPortraitMobile], () => {
  mapStore.setDrawerBottomPx(isPortraitMobile.value ? getDrawerVisibleHeight() : 0)
}, {immediate: true})

watch(isLandscapeDrawerOpen, () => {
  window.dispatchEvent(new Event('resize'))
  setTimeout(() => window.dispatchEvent(new Event('resize')), 280)
})

let pointerId = -1
let startY = 0
let startHeight = 0 // visible height at drag start
let moved = false
let lastY = 0
let lastT = 0
let velocityY = 0 // px/ms, positive = downward
let currentDragHeightPx = 0

function viewportPx() {
  return window.visualViewport?.height ?? window.innerHeight
}

function getDrawerVisibleHeight(): number {
  const vh = viewportPx()
  if (drawerState.value === 'minimized') return MINIMIZED_PX
  if (drawerState.value === 'fullscreen') return vh
  return SNAP_FRAC[drawerState.value] * vh
}

function onPointerDown(e: PointerEvent) {
  if (e.button !== 0 && e.pointerType === 'mouse') return
  const el = drawerEl.value
  if (!el) return
  pointerId = e.pointerId
  startY = e.clientY
  startHeight = getDrawerVisibleHeight()
  currentDragHeightPx = startHeight
  moved = false
  velocityY = 0
  lastY = e.clientY
  lastT = e.timeStamp
  isDragging.value = true
  el.style.transition = 'none'
  // Pin the drawer at its current position immediately so there's no jump on drag start
  const vh = viewportPx()
  el.style.transform = `translateY(${vh - startHeight}px)`
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  if (!isDragging.value || e.pointerId !== pointerId) return
  const el = drawerEl.value
  if (!el) return
  const dy = e.clientY - startY
  if (Math.abs(dy) > 3) moved = true
  const dt = e.timeStamp - lastT
  if (dt > 0) velocityY = (e.clientY - lastY) / dt
  lastY = e.clientY
  lastT = e.timeStamp
  const vh = viewportPx()
  currentDragHeightPx = Math.max(MINIMIZED_PX, Math.min(vh, startHeight - dy))
  el.style.transform = `translateY(${vh - currentDragHeightPx}px)`
}

function endDrag() {
  if (!isDragging.value) return
  const el = drawerEl.value
  const vh = viewportPx()
  const height = currentDragHeightPx || getDrawerVisibleHeight()

  const snapStates: DrawerState[] = ['minimized', 'half', 'fullscreen']
  const snapHeights: Record<DrawerState, number> = {
    minimized: MINIMIZED_PX,
    collapsed: SNAP_FRAC.collapsed * vh,
    half: SNAP_FRAC.half * vh,
    expanded: SNAP_FRAC.expanded * vh,
    fullscreen: vh,
  }

  let best: DrawerState = 'half'

  const FLICK_THRESHOLD = 0.4 // px/ms
  if (!moved) {
    if (drawerState.value === 'minimized' || drawerState.value === 'fullscreen') best = 'half'
    else best = 'minimized'
  } else if (velocityY > FLICK_THRESHOLD) {
    // positive velocityY = moving down = collapsing
    const cur = snapStates.indexOf(drawerState.value)
    best = snapStates[Math.max(0, cur - 1)]!
  } else if (velocityY < -FLICK_THRESHOLD) {
    // negative velocityY = moving up = expanding
    const cur = snapStates.indexOf(drawerState.value)
    best = snapStates[Math.min(snapStates.length - 1, cur + 1)]!
  } else {
    let bestDist = Infinity
    for (const s of snapStates) {
      const d = Math.abs(snapHeights[s] - height)
      if (d < bestDist) {
        bestDist = d
        best = s
      }
    }
  }

  if (el) {
    // Keeping el.style.transform lets the CSS transition animate from wherever the drag ended.
    el.style.transition = ''
  }
  isDragging.value = false
  drawerState.value = best
  pointerId = -1
}

function onPointerUp(e: PointerEvent) {
  if (e.pointerId !== pointerId) return
  const el = e.currentTarget as HTMLElement
  if (el.hasPointerCapture?.(e.pointerId)) {
    try {
      el.releasePointerCapture(e.pointerId)
    } catch { /* noop */
    }
  }
  endDrag()
}

function toggleLandscapeDrawer() {
  isLandscapeDrawerOpen.value = !isLandscapeDrawerOpen.value
}
</script>

<template>
  <main class="app-shell bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
    <OfflinePill :landscape-open="isLandscapeDrawerOpen"/>
    <MapComponent class="app-map"/>
    <SettingsButton :class="{ 'landscape-open': isLandscapeDrawerOpen }"/>
    <NewsButton
      v-if="appSettings.showNews"
      :class="{ 'landscape-open': isLandscapeDrawerOpen }"
    />
    <WeatherButton
      v-if="appSettings.showWeather"
      :top-offset="appSettings.showNews ? '6.25rem' : '3.5rem'"
      :class="{ 'landscape-open': isLandscapeDrawerOpen }"
    />
    <ArcadeToast/>
    <button
      type="button"
      class="landscape-drawer-toggle"
      :class="{ 'is-open': isLandscapeDrawerOpen }"
      :title="t('toggleDrawer')"
      :aria-label="t('toggleDrawer')"
      :aria-expanded="isLandscapeDrawerOpen"
      @click="toggleLandscapeDrawer"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor"
           stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path v-if="isLandscapeDrawerOpen" d="M9 18l6-6-6-6"/>
        <path v-else d="M15 18l-6-6 6-6"/>
      </svg>
    </button>

    <aside
      ref="drawerEl"
      class="app-drawer bg-slate-100 dark:bg-slate-900 shadow-xl/30"
      :class="{ 'is-dragging': isDragging, 'is-landscape-open': isLandscapeDrawerOpen }"
      :style="drawerStyle"
    >
      <div v-if="isPortraitMobile" class="drawer-credits" v-html="attributionHtml"></div>
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
        <GreenFridayBanner/>
        <div class="drawer-view">
          <RouterView v-slot="{ Component }">
            <KeepAlive include="RoutePlanningView">
              <component :is="Component"/>
            </KeepAlive>
          </RouterView>
        </div>
      </div>
    </aside>
  </main>
  <ArcadeTransition/>
</template>

<style scoped>
.app-shell {
  --landscape-drawer-width: min(26rem, 58vw);
  position: relative;
  height: 100dvh;
  width: 100vw;
  overflow: hidden;
}

.app-map {
  position: absolute;
  inset: 0;
  contain: paint style;
  min-height: 0;
  width: auto;
  height: 100dvh;
}

.landscape-drawer-toggle {
  position: fixed;
  top: 50%;
  right: calc(0.625rem + env(safe-area-inset-right));
  transform: translateY(-50%);
  z-index: 4500;
  width: 2.25rem;
  height: 2.25rem;
  display: none;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 0.875rem;
  background: #ffffff;
  color: #334155;
  box-shadow: 0 2px 10px -1px rgba(0, 0, 0, 0.14), 0 1px 3px rgba(0, 0, 0, 0.08);
  cursor: pointer;
  transition: right 250ms cubic-bezier(0.32, 0.72, 0, 1), background 150ms ease, color 150ms ease;
}

.landscape-drawer-toggle:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.landscape-drawer-toggle svg {
  width: 1rem;
  height: 1rem;
}

/* Height stays 100dvh so translateY never triggers a layout reflow on the map behind it. */
.app-drawer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 4000;
  height: 100dvh;
  border-top-left-radius: 1.25rem;
  border-top-right-radius: 1.25rem;
  display: flex;
  flex-direction: column;
  transition: transform 250ms cubic-bezier(0.32, 0.72, 0, 1);
  padding-bottom: env(safe-area-inset-bottom);
}

.app-drawer.is-dragging {
  transition: none;
  will-change: transform;
}

.is-dragging .drawer-scroll {
  contain: layout paint;
  max-height: 100dvh;
}

.is-dragging .drawer-view {
  content-visibility: auto;
  contain-intrinsic-size: auto 300px;
}

.drawer-credits {
  position: absolute;
  bottom: 100%;
  right: 0.5rem;
  padding: 2px 8px;
  background: none;
  border-radius: 6px 6px 0 0;
  font-size: 10px;
  color: #64748b;
  pointer-events: auto;
  white-space: nowrap;
  z-index: 10;
}

.drawer-credits :deep(a) {
  color: #475569;
  text-decoration: none;
}

:root.dark .drawer-credits {
  color: #94a3b8;
}

:root.dark .drawer-credits :deep(a) {
  color: #64748b;
}

.drawer-handle {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-shrink: 0;
  height: 2.75rem;
  width: 100%;
  background: transparent;
  border: 0;
  cursor: grab;
  touch-action: none;
  user-select: none;
}

.drawer-handle:active {
  cursor: grabbing;
}

.drawer-grip {
  width: 3.5rem;
  height: 0.375rem;
  border-radius: 9999px;
  background: #cbd5e1;
}

.drawer-scroll {
  flex: 1 1 auto;
  min-height: 0;
  /* Clamp scroll area to the visually exposed portion so content doesn't hide below the fold. */
  max-height: calc(var(--drawer-visible-h, 100dvh) - 2.75rem);
  display: flex;
  flex-direction: column;
  touch-action: pan-y;
}

.drawer-view {
  flex: 1 1 0;
  min-height: 0;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  touch-action: pan-y;
  overscroll-behavior: contain;
  contain: layout paint;
}

@media (max-width: 1023px) and (orientation: landscape) {
  .app-shell {
    position: relative;
  }

  .landscape-drawer-toggle {
    display: inline-flex;
  }

  .landscape-drawer-toggle.is-open {
    right: calc(var(--landscape-drawer-width) + 0.625rem + env(safe-area-inset-right));
  }

  /* drawerStyle returns {} here, so this CSS transform won't be shadowed by an inline style. */
  .app-drawer {
    position: absolute;
    top: 0;
    right: 0;
    left: auto;
    width: var(--landscape-drawer-width);
    height: 100dvh !important;
    border-radius: 1.25rem 0 0 1.25rem;
    padding-bottom: 0;
    transform: translateX(calc(100% + env(safe-area-inset-right)));
    opacity: 0;
    pointer-events: none;
    z-index: 4400;
    box-shadow: 0 12px 28px rgba(15, 23, 42, 0.24);
    transition: transform 250ms cubic-bezier(0.32, 0.72, 0, 1), opacity 180ms ease;
  }

  .app-drawer.is-landscape-open {
    transform: translateX(0);
    opacity: 1;
    pointer-events: auto;
  }

  .drawer-handle {
    display: none;
  }

  .drawer-scroll {
    max-height: 100dvh;
  }
}

@media (min-width: 1024px) {
  .landscape-drawer-toggle {
    display: none !important;
  }

  .app-shell {
    display: flex;
    flex-direction: row;
    height: 100dvh;
  }

  .app-map {
    position: static;
    flex: 1 1 auto;
  }

  .app-drawer {
    position: relative;
    transform: none;
    flex-shrink: 0;
    width: 30vw;
    height: 100dvh !important;
    border-radius: 0;
    padding-bottom: 0;
    transition: none;
  }

  .drawer-handle {
    display: none;
  }

  .drawer-scroll {
    max-height: 100dvh;
  }
}
</style>
