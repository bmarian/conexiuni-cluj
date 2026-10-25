<script setup lang="ts">
import {computed, nextTick, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useHead} from '@unhead/vue'
import MapComponent from '@/components/MapComponent.vue'
import {mapCreditsHTML} from '@/utils/mapStyles.ts'
import OfflinePill from '@/components/OfflinePill.vue'
import GreenFridayBanner from '@/components/GreenFridayBanner.vue'
import AppToast from '@/components/AppToast.vue'
import KeyboardHelp from '@/components/KeyboardHelp.vue'
import HomeChrome from '@/components/home/HomeChrome.vue'
import MapFabs from '@/components/home/MapFabs.vue'
import SearchOverlay from '@/components/home/SearchOverlay.vue'
import {useRouteUpdatesStore} from '@/stores/routeUpdates.ts'
import {useMapStore} from '@/stores/map.ts'
import {useSettingsStore} from '@/stores/settings.ts'
import {useKbdShortcuts, useKeyboardNav, useKeyboardNavRoot} from '@/composables/useKeyboardNav.ts'
import {cityParams, cityPath} from '@/utils/city.ts'
import {useSheetMetrics} from '@/composables/useSheetMetrics.ts'
import {useSheetGestures, type SheetSnap} from '@/composables/useSheetGestures.ts'
import {useIOSKeyboard} from '@/composables/useIOSKeyboard.ts'
import {useSheetKeyboard} from '@/composables/useSheetKeyboard.ts'
import {useStandaloneRootBackground} from '@/composables/useStandaloneRootBackground.ts'
import {takeUAVisualTransition} from '@/router/index.ts'
import {KeyboardArrowLeftIcon, KeyboardArrowRightIcon} from '@/utils/icons.ts'

const {t} = useI18n()
useHead(() => ({title: t('headHomeTitle', cityParams())}))
const mapStore = useMapStore()
const appSettings = useSettingsStore()
const routeUpdates = useRouteUpdatesStore()
const route = useRoute()
const router = useRouter()
const {keyboardMode, focusSectionSoon} = useKeyboardNav()
useKeyboardNavRoot(router)
const isAdminRoute = computed(() => route.name === 'admin')
const atHome = computed(() => route.name === 'home')

type DrawerState = SheetSnap

const SHEET_HANDLE_PX = 28
const PEEK_CONTENT_PX = 64
const MINIMIZED_PX = SHEET_HANDLE_PX + PEEK_CONTENT_PX
const SHEET_SETTLE_MS = 400
const FABS_FADE_PX = 120

const SNAP_FRAC: Record<DrawerState, number> = {
  minimized: 0,
  half: 0.5,
  fullscreen: 1.0,
}

const shellEl = ref<HTMLElement | null>(null)
const drawerEl = ref<HTMLElement | null>(null)
const chromeEl = ref<{$el: HTMLElement} | null>(null)
const chromeH = ref(0)
const drawerState = ref<DrawerState>('half')
const isLandscapeDrawerOpen = ref(true)
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

type VirtualKeyboard = EventTarget & {overlaysContent: boolean; boundingRect: DOMRect}
// Installed Android PWAs don't move the page for the keyboard; browser tabs do.
const virtualKeyboard = window.matchMedia('(display-mode: standalone)').matches
  ? (navigator as unknown as {virtualKeyboard?: VirtualKeyboard}).virtualKeyboard
  : undefined
const keyboardPx = ref(0)

const onKeyboardGeometryChange = () => {
  keyboardPx.value = virtualKeyboard?.overlaysContent ? virtualKeyboard.boundingRect.height : 0
}

const iosKeyboard = useIOSKeyboard(drawerEl, {enabled: isPortraitMobile, sheetHeight: sheetH, liftTo: safeTop, keyboardPx})
useSheetKeyboard(drawerEl, {
  enabled: isPortraitMobile,
  keyboardPx,
  measureViewport: !virtualKeyboard,
  open: () => {
    drawerState.value = 'fullscreen'
  },
})

useStandaloneRootBackground(
  () => (isPortraitMobile.value && drawerState.value === 'minimized' ? drawerEl.value : drawerEl.value?.querySelector('.drawer-view > *')),
  [() => route.fullPath, drawerState, isPortraitMobile],
)

const addMqlChangeListener = (mql: MediaQueryList, listener: () => void) => {
  mql.addEventListener('change', listener)
  ;(mql as unknown as {addListener?: (cb: () => void) => void}).addListener?.(listener)
}

const removeMqlChangeListener = (mql: MediaQueryList, listener: () => void) => {
  mql.removeEventListener('change', listener)
  ;(mql as unknown as {removeListener?: (cb: () => void) => void}).removeListener?.(listener)
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

watch(
  [isPortraitMobile, isAdminRoute],
  ([portrait, admin]) => {
    if (!virtualKeyboard) return
    virtualKeyboard.overlaysContent = portrait && !admin
    onKeyboardGeometryChange()
  },
  {immediate: true},
)

const drawerStyle = computed(() => {
  if (!isPortraitMobile.value) return {}
  const top = iosKeyboard.fieldFocused.value ? safeTop.value : snapTopPx(drawerState.value)
  return {
    transform: `translateY(${top}px)`,
    '--drawer-visible-h': `${sheetH.value - keyboardPx.value - top}px`,
  }
})

function sheetFollowers(top: number) {
  const half = snapTopPx('half')
  const reach = chromeH.value
  return {
    '--sheet-top': `${top}px`,
    '--fabs-alpha': String(Math.min(1, Math.max(0, (top - (half - FABS_FADE_PX)) / FABS_FADE_PX))),
    '--chrome-alpha': reach <= 0 ? '1' : String(Math.min(1, Math.max(0, (top - reach * 0.25) / reach))),
  }
}

const shellStyle = computed(() => (isPortraitMobile.value ? sheetFollowers(snapTopPx(drawerState.value)) : {}))

function followDrag(top: number) {
  const shell = shellEl.value
  if (!shell) return
  for (const [name, value] of Object.entries(sheetFollowers(top))) shell.style.setProperty(name, value)
}

watch([drawerState, isDragging], async ([, dragging]) => {
  if (dragging) return
  await nextTick()
  setTimeout(() => window.dispatchEvent(new Event('resize')), SHEET_SETTLE_MS)
})

let chromeObserver: ResizeObserver | null = null
watch(chromeEl, (chrome) => {
  chromeObserver?.disconnect()
  chromeH.value = 0
  if (!chrome) return
  chromeObserver = new ResizeObserver(() => {
    chromeH.value = chrome.$el.getBoundingClientRect().bottom
  })
  chromeObserver.observe(chrome.$el)
})

watch(
  [atHome, chromeH],
  ([home, h]) => {
    mapStore.chromeTopPx = home ? h : safeTop.value
  },
  {immediate: true},
)

// Rows rendered in the next `ms` appear in place instead of cascading in.
function settleCascades(ms = 1000) {
  const until = performance.now() + ms
  const tick = (now: number) => {
    for (const animation of document.getAnimations()) {
      if (animation instanceof CSSAnimation && animation.animationName === 'cascade-in') animation.finish()
    }
    if (now < until) requestAnimationFrame(tick)
  }
  requestAnimationFrame(tick)
}

const navBack = ref(false)
const navInstant = ref(false)
let lastPosition = history.state?.position ?? 0
watch(
  () => route.fullPath,
  () => {
    const position = history.state?.position ?? 0
    navBack.value = position < lastPosition
    lastPosition = position
    navInstant.value = takeUAVisualTransition()
    if (navInstant.value) settleCascades()
  },
)

watch([drawerState, isPortraitMobile, sheetH, safeTop, safeBottom], scheduleMapInsetSync, {immediate: true})

function reveal() {
  if (isPortraitMobile.value) {
    if (drawerState.value !== 'minimized') return
    drawerState.value = 'half'
    updateMapInsets()
  } else if (mqlLandscape?.matches && !isLandscapeDrawerOpen.value) {
    isLandscapeDrawerOpen.value = true
  }
}

// Sync, because the map flies to the pick right after and has to aim above the raised drawer.
watch(
  () => mapStore.revealDrawer,
  (requested) => {
    if (!requested) return
    mapStore.revealDrawer = false
    reveal()
  },
  {flush: 'sync'},
)

watch(
  () => route.path,
  () => {
    if (!atHome.value && !isAdminRoute.value) reveal()
  },
)

watch(isLandscapeDrawerOpen, () => {
  scheduleMapInsetSync()
  window.dispatchEvent(new Event('resize'))
  setTimeout(() => window.dispatchEvent(new Event('resize')), SHEET_SETTLE_MS)
})

function snapTopPx(state: DrawerState, kb = keyboardPx.value): number {
  const h = sheetH.value
  const minimizedTop = h - MINIMIZED_PX - Math.max(kb, safeBottom.value)
  if (state === 'minimized') return minimizedTop
  return Math.min(minimizedTop, Math.max(safeTop.value, h * (1 - SNAP_FRAC[state]) - kb))
}

function updateMapInsets() {
  mapStore.setDrawerBottomPx(isPortraitMobile.value ? sheetH.value - snapTopPx(drawerState.value, 0) : 0)

  if (isPortraitMobile.value) {
    mapStore.setDrawerLeftPx(0)
    return
  }

  const drawer = drawerEl.value
  const mapEl = document.querySelector('.app-map') as HTMLElement | null
  if (!drawer || !mapEl) {
    mapStore.setDrawerLeftPx(0)
    return
  }

  const mapRect = mapEl.getBoundingClientRect()
  const drawerRect = drawer.getBoundingClientRect()
  const verticalOverlap = Math.max(0, Math.min(mapRect.bottom, drawerRect.bottom) - Math.max(mapRect.top, drawerRect.top))
  const horizontalOverlap = Math.max(0, Math.min(mapRect.right, drawerRect.right) - Math.max(mapRect.left, drawerRect.left))
  mapStore.setDrawerLeftPx(verticalOverlap > 0 && horizontalOverlap > 2 ? horizontalOverlap : 0)
}

function scheduleMapInsetSync() {
  if (mapInsetRaf) cancelAnimationFrame(mapInsetRaf)
  mapInsetRaf = requestAnimationFrame(() => {
    updateMapInsets()
    mapInsetRaf = 0
  })

  if (mapInsetTimer) clearTimeout(mapInsetTimer)
  mapInsetTimer = setTimeout(updateMapInsets, SHEET_SETTLE_MS)
}

const sheetGestures = useSheetGestures({
  sheet: drawerEl,
  enabled: isPortraitMobile,
  state: drawerState,
  dragging: isDragging,
  topFor: (snap) => snapTopPx(snap),
  onMove: followDrag,
  held: () => iosKeyboard.fieldFocused.value,
})

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

useKbdShortcuts(
  {
    m: cycleDrawerSize,
    o: () => void router.push(cityPath('/settings')),
    n: () => {
      if (routeUpdates.hasFeed) void router.push(cityPath('/news'))
    },
    c: () => {
      void router.push(cityPath('/settings'))
      focusSectionSoon('city')
    },
  },
  {global: true},
)
</script>

<template>
  <RouterView v-if="isAdminRoute" />
  <main v-else ref="shellEl" class="app-shell" :class="{'is-dragging': isDragging, 'is-landscape-open': isLandscapeDrawerOpen}" :style="shellStyle">
    <MapComponent class="app-map" />
    <Transition name="chrome" :css="!navInstant">
      <HomeChrome v-if="atHome" ref="chromeEl" class="home-chrome-place" />
    </Transition>
    <OfflinePill class="offline-place" />
    <MapFabs class="fabs-place" />
    <AppToast />
    <button
      type="button"
      class="fab fab-small side-toggle squish"
      :title="t('toggleDrawer')"
      :aria-label="t('toggleDrawer')"
      :aria-expanded="isLandscapeDrawerOpen"
      @click="toggleLandscapeDrawer"
    >
      <KeyboardArrowLeftIcon v-if="isLandscapeDrawerOpen" class="m-icon" />
      <KeyboardArrowRightIcon v-else class="m-icon" />
    </button>

    <aside
      ref="drawerEl"
      class="app-drawer"
      :class="{'is-dragging': isDragging, 'is-landscape-open': isLandscapeDrawerOpen}"
      :style="drawerStyle"
      data-kbd-root
      @focusin="onDrawerFocusIn"
    >
      <div v-if="isPortraitMobile" class="drawer-credits" v-html="mapCreditsHTML()"></div>
      <!-- touchend.prevent: iOS fires the tap's click late, on whatever the moving drawer slid under the finger. -->
      <div
        class="drawer-handle"
        role="button"
        tabindex="0"
        :aria-label="t('toggleDrawer')"
        @pointerdown="sheetGestures.onHandleDown"
        @pointermove="sheetGestures.onHandleMove"
        @pointerup="sheetGestures.onHandleUp"
        @pointercancel="sheetGestures.onHandleUp"
        @touchend.prevent
        @keydown.enter.space.prevent="cycleDrawerSize"
      >
        <span class="drawer-grip"></span>
      </div>
      <div class="drawer-scroll">
        <GreenFridayBanner v-if="appSettings.showGreenFriday" />
        <div class="drawer-view">
          <!-- RouteView only loads on mount, so another line needs a fresh instance. -->
          <!-- Without CSS, out-in re-renders mid-patch and crashes, so an instant swap uses the default mode. -->
          <RouterView v-slot="{Component, route: viewRoute}">
            <Transition :name="navBack ? 'page-back' : 'page'" :css="!navInstant" :mode="navInstant ? 'default' : 'out-in'">
              <KeepAlive include="RoutePlanningView">
                <component
                  :is="Component"
                  :key="`${viewRoute.params.city}/${viewRoute.name === 'route' || viewRoute.name === 'route-id' ? `route-${viewRoute.params.line ?? viewRoute.params.routeId}` : String(viewRoute.name)}`"
                />
              </KeepAlive>
            </Transition>
          </RouterView>
        </div>
      </div>
    </aside>
    <SearchOverlay />
  </main>
  <KeyboardHelp v-if="!isAdminRoute" />
</template>

<style scoped>
@property --sheet-top {
  syntax: '<length>';
  inherits: true;
  initial-value: 0px;
}

@property --fabs-alpha {
  syntax: '<number>';
  inherits: true;
  initial-value: 1;
}

@property --chrome-alpha {
  syntax: '<number>';
  inherits: true;
  initial-value: 1;
}

.app-shell {
  --panel-w: min(420px, 58vw);
  --panel-shown: 0px;
  position: relative;
  height: 100dvh;
  width: 100vw;
  overflow: hidden;
  background: var(--md-surface);
  color: var(--md-on-surface);
  transition:
    --sheet-top var(--spring-sheet-ms) var(--spring-sheet),
    --fabs-alpha var(--spring-sheet-ms) var(--spring-sheet),
    --chrome-alpha var(--spring-sheet-ms) var(--spring-sheet);
}

.app-shell.is-dragging {
  transition: none;
}

.app-map {
  position: absolute;
  inset: 0;
  contain: paint style;
  min-height: 0;
  width: auto;
  height: 100dvh;
}

.home-chrome-place {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 3000;
  opacity: var(--chrome-alpha);
}

.chrome-enter-active {
  transition: opacity 200ms linear;
}

.chrome-leave-active {
  transition: opacity 120ms linear;
}

.chrome-enter-from,
.chrome-leave-to {
  opacity: 0;
}

.fabs-place {
  position: absolute;
  right: 16px;
  top: calc(var(--sheet-top) - 28px);
  z-index: 3500;
  translate: 0 -100%;
  opacity: var(--fabs-alpha);
}

.offline-place {
  position: absolute;
  top: calc(12px + var(--safe-top));
  left: 50%;
  translate: -50% 0;
  z-index: 3200;
}

.side-toggle {
  display: none;
}

.app-drawer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 4000;
  height: 100dvh;
  border-top-left-radius: 28px;
  border-top-right-radius: 28px;
  display: flex;
  flex-direction: column;
  background: var(--md-surface-container-low);
  color: var(--md-on-surface);
  box-shadow:
    0 -1px 3px rgb(0 0 0 / 0.12),
    0 -4px 12px rgb(0 0 0 / 0.08);
  transition: transform var(--spring-sheet-ms) var(--spring-sheet);
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
  left: 0;
  padding: 0 0 2px 8px;
  font-size: 10px;
  line-height: 16px;
  font-weight: 500;
  letter-spacing: 0.5px;
  color: var(--md-on-surface-variant);
  pointer-events: auto;
  white-space: nowrap;
  z-index: 10;
}

.drawer-credits :deep(a) {
  color: inherit;
  text-decoration: none;
}

.drawer-handle {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-shrink: 0;
  height: 28px;
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
  width: 36px;
  height: 4px;
  border-radius: 2px;
  background: color-mix(in srgb, var(--md-on-surface-variant) 40%, transparent);
}

.drawer-scroll {
  flex: 1 1 auto;
  min-height: 0;
  /* Clamp scroll area to the visually exposed portion so content doesn't hide below the fold. */
  max-height: calc(var(--drawer-visible-h, 100dvh) - 28px);
  display: flex;
  flex-direction: column;
  touch-action: pan-y;
}

.drawer-view {
  flex: 1 1 0;
  min-height: 0;
  overflow-y: auto;
  overflow-x: clip;
  -webkit-overflow-scrolling: touch;
  touch-action: pan-y;
  overscroll-behavior: contain;
  contain: layout paint;
}

.drawer-view::after {
  content: '';
  display: block;
  height: env(safe-area-inset-bottom);
}

.page-enter-active,
.page-back-enter-active {
  transition:
    opacity 210ms cubic-bezier(0, 0, 0.2, 1) 90ms,
    translate var(--spring-page-ms) var(--spring-page) 90ms,
    scale var(--spring-page-ms) var(--spring-page) 90ms;
}

.page-leave-active,
.page-back-leave-active {
  transition:
    opacity 90ms cubic-bezier(0.4, 0, 1, 1),
    translate 90ms cubic-bezier(0.4, 0, 1, 1),
    scale 90ms cubic-bezier(0.4, 0, 1, 1);
}

.page-enter-from {
  opacity: 0;
  translate: 25% 0;
  scale: 0.9;
}

.page-leave-to {
  opacity: 0;
  translate: -12.5% 0;
  scale: 0.95;
}

.page-back-enter-from {
  opacity: 0;
  translate: -25% 0;
  scale: 0.9;
}

.page-back-leave-to {
  opacity: 0;
  translate: 12.5% 0;
  scale: 0.95;
}

@media (prefers-reduced-motion: reduce) {
  .page-enter-from,
  .page-leave-to,
  .page-back-enter-from,
  .page-back-leave-to {
    translate: none;
    scale: none;
  }
}

@media (max-width: 1023px) and (orientation: landscape), (min-width: 1024px) {
  .home-chrome-place {
    left: var(--panel-shown);
    right: auto;
    width: min(520px, calc(100vw - var(--panel-shown)));
    opacity: 1;
    transition: left var(--spring-sheet-ms) var(--spring-sheet);
  }

  .fabs-place {
    top: auto;
    bottom: calc(16px + env(safe-area-inset-bottom));
    translate: none;
    opacity: 1;
  }

  .offline-place {
    left: calc(var(--panel-shown) + (100vw - var(--panel-shown)) / 2);
  }

  .drawer-handle {
    display: none;
  }

  .drawer-scroll {
    max-height: 100dvh;
  }
}

@media (max-width: 1023px) and (orientation: landscape) {
  .app-shell.is-landscape-open {
    --panel-shown: var(--panel-w);
    --map-overlap: var(--panel-w);
  }

  .side-toggle {
    display: inline-flex;
    position: absolute;
    top: 50%;
    left: calc(var(--panel-shown) + 8px + env(safe-area-inset-left));
    z-index: 4500;
    translate: 0 -50%;
    transition:
      left var(--spring-sheet-ms) var(--spring-sheet),
      scale var(--spring-press-ms) var(--spring-press);
  }

  /* drawerStyle returns {} here, so this CSS transform won't be shadowed by an inline style. */
  .app-drawer {
    position: absolute;
    top: 0;
    left: 0;
    right: auto;
    width: var(--panel-w);
    height: 100dvh !important;
    border-radius: 0 28px 28px 0;
    padding-left: env(safe-area-inset-left);
    transform: translateX(calc(-100% - env(safe-area-inset-left)));
    pointer-events: none;
    z-index: 4400;
    box-shadow:
      1px 0 3px rgb(0 0 0 / 0.12),
      4px 0 12px rgb(0 0 0 / 0.08);
  }

  .app-drawer.is-landscape-open {
    transform: translateX(0);
    pointer-events: auto;
  }
}

@media (min-width: 1024px) {
  .app-shell {
    --panel-w: max(30vw, 360px);
    --panel-shown: var(--panel-w);
    display: flex;
    flex-direction: row;
  }

  .app-map {
    position: static;
    flex: 1 1 auto;
  }

  .app-drawer {
    position: relative;
    order: -1;
    transform: none;
    flex-shrink: 0;
    width: var(--panel-w);
    height: 100dvh !important;
    border-radius: 0;
    box-shadow:
      1px 0 3px rgb(0 0 0 / 0.12),
      4px 0 12px rgb(0 0 0 / 0.08);
    transition: none;
    z-index: 4400;
  }
}
</style>
