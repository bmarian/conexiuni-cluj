<script setup lang="ts">
import {computed, defineAsyncComponent, nextTick, onMounted, onUnmounted, ref, watch} from "vue"
import {useI18n} from "vue-i18n"
import {useRoute, useRouter} from "vue-router"
import {useHead} from "@unhead/vue"
import MapComponent from "@/components/MapComponent.vue"
import {mapCreditsHTML} from "@/utils/mapStyles.ts"
import SettingsButton from "@/components/SettingsButton.vue"
import OfflinePill from "@/components/OfflinePill.vue"
import GreenFridayBanner from "@/components/GreenFridayBanner.vue"
import AndroidAppBanner from "@/components/AndroidAppBanner.vue"
import HostMoveBanner from "@/components/HostMoveBanner.vue"
import AppToast from "@/components/AppToast.vue"
import KeyboardHelp from "@/components/KeyboardHelp.vue"
import BackgroundTracking from "@/components/BackgroundTracking.vue"
import ReminderSheet from "@/components/ReminderSheet.vue"
import {useRemindersStore} from "@/stores/reminders.ts"
import {useMapStore} from "@/stores/map.ts"
import {useSettingsStore} from "@/stores/settings.ts"
import {useOnline} from "@/composables/useOnline.ts"
import {useKbdShortcuts, useKeyboardNav, useKeyboardNavRoot} from "@/composables/useKeyboardNav.ts"
import {useRouteUpdatesStore} from "@/stores/routeUpdates"
import {cityParams} from "@/utils/city.ts"
import {isTypingTarget} from "@/utils/keyboardFocus.ts"
import {useSheetMetrics} from "@/composables/useSheetMetrics.ts"
import {useIOSKeyboard} from "@/composables/useIOSKeyboard.ts"
import {useStandaloneRootBackground} from "@/composables/useStandaloneRootBackground.ts"

const CityButton = defineAsyncComponent(() => import("@/components/CityButton.vue"))
const NewsButton = defineAsyncComponent(() => import("@/components/NewsButton.vue"))

const {t} = useI18n()
useHead(() => ({title: t('headHomeTitle', cityParams())}))
const mapStore = useMapStore()
const appSettings = useSettingsStore()
const reminders = useRemindersStore()
const {isOnline} = useOnline()
const route = useRoute()
const {keyboardMode} = useKeyboardNav()
useKeyboardNavRoot(useRouter())
const isAdminRoute = computed(() => route.name === 'admin')
const routeUpdates = useRouteUpdatesStore()
const showNews = computed(() => routeUpdates.hasFeed && isOnline.value)

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
let mapInsetRaf = 0
let mapInsetTimer: ReturnType<typeof setTimeout> | null = null
const {height: sheetH, safeTop, safeBottom} = useSheetMetrics(drawerEl)

// Landscape and desktop let CSS handle the transform, so drawerStyle must not set one there.
const isPortraitMobile = ref(false)
let mqlLandscape: MediaQueryList | null = null
let mqlDesktop: MediaQueryList | null = null
let keyboardOpen = false
let maxViewportHeight = 0
let vvResizeHandler: (() => void) | null = null

type VirtualKeyboard = EventTarget & { overlaysContent: boolean, boundingRect: DOMRect }
// Installed Android PWAs don't move the page for the keyboard; browser tabs do.
const virtualKeyboard = window.matchMedia('(display-mode: standalone)').matches
  ? (navigator as unknown as { virtualKeyboard?: VirtualKeyboard }).virtualKeyboard
  : undefined
const keyboardPx = ref(0)

const onKeyboardGeometryChange = () => {
  keyboardPx.value = virtualKeyboard?.overlaysContent ? virtualKeyboard.boundingRect.height : 0
}

const iosKeyboard = useIOSKeyboard(drawerEl, {enabled: isPortraitMobile, sheetHeight: sheetH, liftTo: safeTop, keyboardPx})

useStandaloneRootBackground(
  () => isPortraitMobile.value && drawerState.value === 'minimized'
    ? drawerEl.value
    : drawerEl.value?.querySelector('.drawer-view > *'),
  [() => route.fullPath, drawerState, isPortraitMobile],
)

const addMqlChangeListener = (mql: MediaQueryList, listener: () => void) => {
  mql.addEventListener('change', listener)
  ;(mql as unknown as { addListener?: (cb: () => void) => void }).addListener?.(listener)
}

const removeMqlChangeListener = (mql: MediaQueryList, listener: () => void) => {
  mql.removeEventListener('change', listener)
  ;(mql as unknown as { removeListener?: (cb: () => void) => void }).removeListener?.(listener)
}

const updatePortraitMobile = (force = false) => {
  if (!force && keyboardOpen) return
  isPortraitMobile.value = !(mqlLandscape?.matches || mqlDesktop?.matches)
}

onMounted(() => {
  mqlLandscape = window.matchMedia('(max-width: 1023px) and (orientation: landscape)')
  mqlDesktop = window.matchMedia('(min-width: 1024px)')
  addMqlChangeListener(mqlLandscape, updatePortraitMobile)
  addMqlChangeListener(mqlDesktop, updatePortraitMobile)

  const vv = window.visualViewport
  if (vv) {
    maxViewportHeight = vv.height
    vvResizeHandler = () => {
      const mobile = !mqlDesktop?.matches
      if (!mobile) {
        keyboardOpen = false
        maxViewportHeight = vv.height
        updatePortraitMobile(true)
        scheduleMapInsetSync()
        return
      }
      maxViewportHeight = Math.max(maxViewportHeight, vv.height)
      const shrink = maxViewportHeight - vv.height
      keyboardOpen = shrink > Math.max(160, maxViewportHeight * 0.22)
      if (!keyboardOpen) {
        maxViewportHeight = vv.height
        updatePortraitMobile(true)
      }
      scheduleMapInsetSync()
    }
    vv.addEventListener('resize', vvResizeHandler, {passive: true})
  }

  virtualKeyboard?.addEventListener('geometrychange', onKeyboardGeometryChange)

  updatePortraitMobile()
  scheduleMapInsetSync()
  window.addEventListener('resize', scheduleMapInsetSync, {passive: true})
})

onUnmounted(() => {
  if (mqlLandscape) {
    removeMqlChangeListener(mqlLandscape, updatePortraitMobile)
  }
  if (mqlDesktop) {
    removeMqlChangeListener(mqlDesktop, updatePortraitMobile)
  }
  if (vvResizeHandler) {
    window.visualViewport?.removeEventListener('resize', vvResizeHandler)
  }
  virtualKeyboard?.removeEventListener('geometrychange', onKeyboardGeometryChange)
  window.removeEventListener('resize', scheduleMapInsetSync)
  if (mapInsetRaf) cancelAnimationFrame(mapInsetRaf)
  if (mapInsetTimer) clearTimeout(mapInsetTimer)
})

watch([isPortraitMobile, isAdminRoute], ([portrait, admin]) => {
  if (!virtualKeyboard) return
  virtualKeyboard.overlaysContent = portrait && !admin
  onKeyboardGeometryChange()
}, {immediate: true})

const drawerStyle = computed(() => {
  if (!isPortraitMobile.value) return {}
  const top = iosKeyboard.fieldFocused.value ? safeTop.value : snapTopPx(drawerState.value)
  return {
    transform: `translateY(${top}px)`,
    '--drawer-visible-h': `${sheetH.value - keyboardPx.value - top}px`,
  }
})

watch([drawerState, isDragging], async ([, dragging]) => {
  if (dragging) return
  await nextTick()
  setTimeout(() => window.dispatchEvent(new Event('resize')), 280)
})

watch([drawerState, isPortraitMobile, sheetH, safeTop, safeBottom], scheduleMapInsetSync, {immediate: true})

// Sync, because the map flies to the pick right after and has to aim above the raised drawer.
watch(() => mapStore.revealDrawer, (reveal) => {
  if (!reveal) return
  mapStore.revealDrawer = false
  if (!isPortraitMobile.value || drawerState.value !== 'minimized') return
  drawerState.value = 'half'
  updateMapInsets()
}, {flush: 'sync'})

watch(isLandscapeDrawerOpen, () => {
  scheduleMapInsetSync()
  window.dispatchEvent(new Event('resize'))
  setTimeout(() => window.dispatchEvent(new Event('resize')), 280)
})

let pointerId = -1
let startY = 0
let startTop = 0
let moved = false
let lastY = 0
let lastT = 0
let velocityY = 0 // px/ms, positive = downward
let currentDragTopPx = 0

function snapTopPx(state: DrawerState, kb = keyboardPx.value): number {
  const h = sheetH.value
  const minimizedTop = h - MINIMIZED_PX - Math.max(kb, safeBottom.value)
  if (state === 'minimized') return minimizedTop
  return Math.min(minimizedTop, Math.max(safeTop.value, h * (1 - SNAP_FRAC[state]) - kb))
}

function updateMapInsets() {
  mapStore.setDrawerBottomPx(isPortraitMobile.value ? sheetH.value - snapTopPx(drawerState.value, 0) : 0)

  if (isPortraitMobile.value) {
    mapStore.setDrawerRightPx(0)
    return
  }

  const drawer = drawerEl.value
  const mapEl = document.querySelector('.app-map') as HTMLElement | null
  if (!drawer || !mapEl) {
    mapStore.setDrawerRightPx(0)
    return
  }

  const mapRect = mapEl.getBoundingClientRect()
  const drawerRect = drawer.getBoundingClientRect()
  const verticalOverlap = Math.max(0, Math.min(mapRect.bottom, drawerRect.bottom) - Math.max(mapRect.top, drawerRect.top))
  const horizontalOverlap = Math.max(0, Math.min(mapRect.right, drawerRect.right) - Math.max(mapRect.left, drawerRect.left))
  mapStore.setDrawerRightPx(verticalOverlap > 0 && horizontalOverlap > 2 ? horizontalOverlap : 0)
}

function scheduleMapInsetSync() {
  if (mapInsetRaf) cancelAnimationFrame(mapInsetRaf)
  mapInsetRaf = requestAnimationFrame(() => {
    updateMapInsets()
    mapInsetRaf = 0
  })

  if (mapInsetTimer) clearTimeout(mapInsetTimer)
  mapInsetTimer = setTimeout(updateMapInsets, 280)
}

function onPointerDown(e: PointerEvent) {
  if (e.button !== 0 && e.pointerType === 'mouse') return
  const el = drawerEl.value
  if (!el) return
  const active = document.activeElement
  if (isTypingTarget(active) && el.contains(active)) (active as HTMLElement).blur()
  pointerId = e.pointerId
  startY = e.clientY
  startTop = snapTopPx(drawerState.value)
  currentDragTopPx = startTop
  moved = false
  velocityY = 0
  lastY = e.clientY
  lastT = e.timeStamp
  isDragging.value = true
  el.style.transition = 'none'
  // Pin the drawer where it is so the drag doesn't jump.
  el.style.transform = `translateY(${startTop}px)`
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
  currentDragTopPx = Math.min(snapTopPx('minimized'), Math.max(safeTop.value, startTop + dy))
  el.style.transform = `translateY(${currentDragTopPx}px)`
}

function endDrag() {
  if (!isDragging.value) return
  const el = drawerEl.value
  const top = currentDragTopPx

  const snapStates: DrawerState[] = ['minimized', 'half', 'fullscreen']

  let best: DrawerState = 'half'

  if (performance.now() - lastT > 100) velocityY = 0

  const FLICK_THRESHOLD = 0.4 // px/ms
  if (!moved) {
    if (drawerState.value === 'minimized' || drawerState.value === 'fullscreen') best = 'half'
    else best = 'minimized'
  } else if (velocityY > FLICK_THRESHOLD) {
    const cur = snapStates.indexOf(drawerState.value)
    best = snapStates[Math.max(0, cur - 1)]!
  } else if (velocityY < -FLICK_THRESHOLD) {
    const cur = snapStates.indexOf(drawerState.value)
    best = snapStates[Math.min(snapStates.length - 1, cur + 1)]!
  } else {
    let bestDist = Infinity
    for (const s of snapStates) {
      const d = Math.abs(snapTopPx(s) - top)
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

function cycleDrawerSize() {
  if (isPortraitMobile.value) {
    const next: Partial<Record<DrawerState, DrawerState>> = {half: 'fullscreen', fullscreen: 'minimized'}
    drawerState.value = next[drawerState.value] ?? 'half'
  } else if (mqlLandscape?.matches) {
    toggleLandscapeDrawer()
  }
}

function onDrawerFocusIn(e: FocusEvent) {
  if (!keyboardMode.value) return
  if (isPortraitMobile.value && drawerState.value === 'minimized') drawerState.value = 'half'
  else if (mqlLandscape?.matches && !isLandscapeDrawerOpen.value) isLandscapeDrawerOpen.value = true
  else return
  const target = e.target as HTMLElement
  void nextTick(() => {
    if (document.activeElement === target) target.scrollIntoView({block: 'nearest'})
  })
}

useKbdShortcuts({m: cycleDrawerSize}, {global: true})
</script>

<template>
  <RouterView v-if="isAdminRoute"/>
  <main v-else class="app-shell bg-slate-100 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
    <OfflinePill :landscape-open="isLandscapeDrawerOpen"/>
    <MapComponent class="app-map"/>
    <SettingsButton
      :class="{ 'landscape-open': isLandscapeDrawerOpen }"
      :style="{ '--controls-row-index': 0 }"
    />
    <NewsButton
      v-if="showNews"
      :class="{ 'landscape-open': isLandscapeDrawerOpen }"
      :style="{ '--controls-row-index': 1 }"
    />
    <CityButton
      :top-offset="showNews ? '6.25rem' : '3.5rem'"
      :class="{ 'landscape-open': isLandscapeDrawerOpen }"
      :style="{ '--controls-row-index': showNews ? 2 : 1 }"
    />
    <AppToast :landscape-open="isLandscapeDrawerOpen"/>
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
      data-kbd-root
      @focusin="onDrawerFocusIn"
    >
      <div v-if="isPortraitMobile" class="drawer-credits" v-html="mapCreditsHTML()"></div>
      <!-- touchend.prevent: iOS fires the tap's click late, on whatever the moving drawer slid under the finger. -->
      <div
        class="drawer-handle lg:hidden"
        role="button"
        tabindex="0"
        aria-label="Drag or tap to resize panel"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
        @touchend.prevent
        @keydown.enter.space.prevent="cycleDrawerSize"
      >
        <span class="drawer-grip"></span>
      </div>
      <div class="drawer-scroll">
        <GreenFridayBanner v-if="appSettings.showGreenFriday"/>
        <HostMoveBanner/>
        <AndroidAppBanner/>
        <div class="drawer-view">
          <!-- RouteView only loads on mount, so another line needs a fresh instance. -->
          <RouterView v-slot="{ Component, route: viewRoute }">
            <KeepAlive include="RoutePlanningView">
              <component :is="Component"
                         :key="`${viewRoute.params.city}/${viewRoute.name === 'route' ? `route-${viewRoute.params.routeId}` : String(viewRoute.name)}`"/>
            </KeepAlive>
          </RouterView>
        </div>
      </div>
    </aside>
  </main>
  <KeyboardHelp v-if="!isAdminRoute"/>
  <BackgroundTracking v-if="!isAdminRoute"/>
  <ReminderSheet v-if="reminders.editing" :key="`${reminders.editing.stopId}:${reminders.editing.tripId}`" :target="reminders.editing" @close="reminders.closeSheet()"/>
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
  will-change: transform;
}

.app-drawer.is-dragging {
  transition: none;
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

.drawer-view::after {
  content: "";
  display: block;
  height: env(safe-area-inset-bottom);
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
    padding-right: env(safe-area-inset-right);
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
