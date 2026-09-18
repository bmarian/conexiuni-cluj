import {nextTick, onActivated, onDeactivated, onMounted, onUnmounted, readonly, ref, watch, type WatchSource} from 'vue'
import type {Router} from 'vue-router'
import {
  activateItem,
  entryItem,
  findItem,
  firstItem,
  focusElement,
  focusItem,
  isEmptyField,
  isNativeActivatable,
  isTypingTarget,
  itemOf,
  neighborOf,
  sectionEntry,
  stepInline,
  stepSection,
  stepVertical,
} from '@/utils/keyboardFocus.ts'

type Handler = (e: KeyboardEvent) => void | boolean
type ShortcutMap = Record<string, Handler>

interface Closer {
  active: () => boolean
  close: () => void
}

interface Layer extends Closer {
  el: () => HTMLElement | null
  capture: boolean
  returnTo: HTMLElement | null
}

interface ShortcutEntry {
  keys: ShortcutMap
  global: boolean
}

const NAV_KEYS: Record<string, ['v' | 'h', 1 | -1]> = {
  j: ['v', 1],
  k: ['v', -1],
  ArrowDown: ['v', 1],
  ArrowUp: ['v', -1],
  h: ['h', -1],
  l: ['h', 1],
  ArrowLeft: ['h', -1],
  ArrowRight: ['h', 1],
}

const BACKSPACE_PAUSE_MS = 500

const keyboardMode = ref(false)
const closers: Closer[] = []
const layers: Layer[] = []
const shortcuts: ShortcutEntry[] = []
const savedFocus = new Map<string, string>()
let navSeq = 0
let pendingFocus: string | null = null
let lastFieldBackspace = -Infinity
let router: Router | null = null

function setKeyboardMode(on: boolean) {
  if (keyboardMode.value === on) return
  keyboardMode.value = on
  document.documentElement.toggleAttribute('data-kbd', on)
}

function addOnce<T>(list: T[], item: T) {
  if (!list.includes(item)) list.push(item)
}

function remove<T>(list: T[], item: T) {
  const i = list.indexOf(item)
  if (i >= 0) list.splice(i, 1)
}

function whileActive(add: () => void, drop: () => void) {
  onMounted(add)
  onActivated(add)
  onDeactivated(drop)
  onUnmounted(drop)
}

function rootEl(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-kbd-root]')
}

function focusEntryIn(scope: HTMLElement): boolean {
  const item = entryItem(scope) ?? firstItem(scope)
  if (item) focusItem(item)
  return !!item
}

function focusedKey(): string | null {
  const root = rootEl()
  return root ? itemOf(document.activeElement, root)?.getAttribute('data-kbd-item') ?? null : null
}

function focusKey(key: string): boolean {
  const root = rootEl()
  const item = root ? findItem(root, key) : null
  if (item) focusItem(item)
  return !!item
}

function focusSection(name: string): boolean {
  const root = rootEl()
  const item = root ? sectionEntry(root, name) : undefined
  if (item) focusItem(item)
  return !!item
}

function focusSectionSoon(name: string, timeoutMs = 1500) {
  const seq = navSeq
  const until = performance.now() + timeoutMs
  const tick = () => {
    if (seq !== navSeq || !keyboardMode.value || focusSection(name) || performance.now() > until) return
    requestAnimationFrame(tick)
  }
  void nextTick(tick)
}

function focusEntry(): boolean {
  const root = rootEl()
  return root ? focusEntryIn(root) : false
}

function focusNext(): boolean {
  const root = rootEl()
  const next = root ? stepVertical(root, document.activeElement, 1) : undefined
  if (next) focusItem(next)
  return !!next
}

function neighborKey(): string | null {
  const root = rootEl()
  const cur = root ? itemOf(document.activeElement, root) : null
  return cur && root ? neighborOf(root, cur)?.getAttribute('data-kbd-item') ?? null : null
}

function closeTop(): boolean {
  for (let i = closers.length - 1; i >= 0; i--) {
    const closer = closers[i]!
    if (closer.active()) {
      closer.close()
      return true
    }
  }
  return false
}

function closeLayers() {
  for (const layer of [...layers].reverse()) {
    if (!layer.capture) layer.close()
  }
}

function goBack() {
  if (!router || router.currentRoute.value.name === 'home') return
  if (history.state?.back) router.back()
  else void router.push({name: 'home'})
}

function goHome() {
  closeLayers()
  if (router?.currentRoute.value.name !== 'home') void router?.push({name: 'home'})
  else focusEntry()
}

function focusSearch() {
  closeLayers()
  if (router?.currentRoute.value.name === 'home') {
    focusKey('search')
    return
  }
  pendingFocus = 'search'
  void router?.push({name: 'home'})
}

function move(scope: HTMLElement, target: Element | null, kind: 'v' | 'h', dir: 1 | -1) {
  const next = kind === 'v' ? stepVertical(scope, target, dir) : stepInline(scope, target, dir)
  if (next) {
    focusItem(next)
    return
  }
  if (kind === 'h' || itemOf(target, scope)) return
  if (!focusEntryIn(scope)) scope.scrollBy({top: dir * 80})
}

// Backspaces that keep coming after the field empties are still deleting, not leaving.
function isDeliberateBackspace(e: KeyboardEvent, field: Element | null) {
  const now = performance.now()
  const paused = now - lastFieldBackspace >= BACKSPACE_PAUSE_MS
  lastFieldBackspace = now
  return paused && !e.repeat && isEmptyField(field)
}

function onKeydown(e: KeyboardEvent) {
  if (e.defaultPrevented || e.isComposing || e.ctrlKey || e.metaKey) return
  if (!rootEl()) return
  const target = e.target instanceof Element ? e.target : null
  if (document.querySelector('.dp__outer_menu_wrap')) return

  const top = layers[layers.length - 1]
  if (top?.capture) {
    if (e.key === 'Escape' || (e.key === 'Backspace' && !e.repeat)) {
      e.preventDefault()
      top.close()
    }
    return
  }

  const scope = top ? top.el() : rootEl()
  if (!scope) return
  const key = e.key
  const act = () => {
    e.preventDefault()
    navSeq++
    setKeyboardMode(true)
  }

  if (key === 'Tab') {
    act()
    const next = stepSection(scope, target, e.shiftKey ? -1 : 1)
    if (next) focusItem(next)
    else focusEntryIn(scope)
    return
  }

  if (isTypingTarget(target)) {
    const wantsClose = key === 'Escape' || (key === 'Backspace' && isDeliberateBackspace(e, target))
    if (wantsClose && closeTop()) act()
    return
  }

  if (e.altKey && /^[a-z]$/i.test(key)) return

  if (target instanceof HTMLSelectElement) {
    if (key.startsWith('Arrow')) return
    // Letters would otherwise change the selected option through type-ahead.
    if (key.length === 1) e.preventDefault()
  }

  if (key === 'Enter' || key === ' ') {
    if (e.repeat) {
      e.preventDefault()
      return
    }
    if (top && target && !scope.contains(target)) {
      e.preventDefault()
      return
    }
    const item = itemOf(target, scope)
    if (!item || isNativeActivatable(target)) return
    act()
    activateItem(item)
    return
  }

  const nav = e.shiftKey ? undefined : NAV_KEYS[key]
  if (nav) {
    if (key.startsWith('Arrow') && target?.closest('.leaflet-container')) return
    act()
    move(scope, target, nav[0], nav[1])
    return
  }

  if (key === 'Escape' || key === 'Backspace') {
    if (e.repeat) return
    act()
    if (!closeTop()) goBack()
    return
  }

  if (e.repeat) return
  const shortcut = e.shiftKey && key === 'ArrowDown' ? 'J' : e.shiftKey && key === 'ArrowUp' ? 'K' : key
  for (let i = shortcuts.length - 1; i >= 0; i--) {
    const entry = shortcuts[i]!
    if (top && !entry.global) continue
    const handler = entry.keys[shortcut]
    if (!handler) continue
    navSeq++
    setKeyboardMode(true)
    if (handler(e) === false) continue
    e.preventDefault()
    return
  }
}

function onPointerDown() {
  navSeq++
  setKeyboardMode(false)
}

function restoreFocus(target: string | null, saved: string | null) {
  const seq = navSeq
  const started = performance.now()
  const tick = () => {
    if (seq !== navSeq || !keyboardMode.value) return
    const root = rootEl()
    if (!root) return
    const active = document.activeElement
    if (!target && active && active !== document.body && root.contains(active)) return
    const waited = performance.now() - started
    const wanted = target ?? saved
    if (wanted) {
      const item = findItem(root, wanted)
      if (item) return focusItem(item)
      if (waited < 1500) return void requestAnimationFrame(tick)
    }
    const entry = entryItem(root, waited < 3000)
    if (entry) return focusItem(entry)
    if (waited > 4000) {
      const first = firstItem(root)
      if (first) focusItem(first)
      return
    }
    requestAnimationFrame(tick)
  }
  void nextTick(tick)
}

export function useKeyboardNavRoot(appRouter: Router) {
  router = appRouter
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('pointerdown', onPointerDown, {capture: true, passive: true})

  const removeBefore = appRouter.beforeEach((_to, from) => {
    const key = focusedKey()
    if (key) savedFocus.set(from.fullPath, key)
  })

  const removeAfter = appRouter.afterEach((to, from, failure) => {
    if (failure) return
    const target = pendingFocus
    pendingFocus = null
    if (!keyboardMode.value) return
    if (to.name === from.name && !target) return
    restoreFocus(target, savedFocus.get(to.fullPath) ?? null)
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', onKeydown)
    window.removeEventListener('pointerdown', onPointerDown, {capture: true})
    removeBefore()
    removeAfter()
    router = null
  })

  useKbdShortcuts({g: goHome, s: focusSearch, '/': focusSearch}, {global: true})
}

export function useKbdShortcuts(keys: ShortcutMap, options: { global?: boolean } = {}) {
  const entry: ShortcutEntry = {keys, global: !!options.global}
  whileActive(() => addOnce(shortcuts, entry), () => remove(shortcuts, entry))
}

export function useKbdEscape(active: () => boolean, close: () => void) {
  const closer: Closer = {active, close}
  whileActive(() => addOnce(closers, closer), () => remove(closers, closer))
}

export function useKbdLayer(
  open: WatchSource<boolean>,
  options: { el?: () => HTMLElement | null, close: () => void, capture?: boolean },
) {
  const layer: Layer = {
    active: () => true,
    close: options.close,
    el: options.el ?? (() => null),
    capture: !!options.capture,
    returnTo: null,
  }

  function push() {
    if (layers.includes(layer)) return
    layer.returnTo = document.activeElement instanceof HTMLElement ? document.activeElement : null
    layers.push(layer)
    addOnce(closers, layer)
  }

  function focusInside() {
    const el = layer.el()
    if (!keyboardMode.value || layer.capture || !el || layers[layers.length - 1] !== layer) return
    if (!focusEntryIn(el)) focusElement(el)
  }

  function pop() {
    if (!layers.includes(layer)) return
    remove(layers, layer)
    remove(closers, layer)
    if (!keyboardMode.value) return
    const active = document.activeElement
    const lost = !active || active === document.body || !!layer.el()?.contains(active)
    const back = layer.returnTo
    if (lost && back?.isConnected && back !== document.body) {
      focusElement(back)
      back.scrollIntoView({block: 'nearest'})
    }
  }

  watch(open, isOpen => isOpen ? push() : pop(), {flush: 'sync', immediate: true})
  watch(open, isOpen => isOpen && focusInside(), {flush: 'post'})
  onDeactivated(pop)
  onUnmounted(pop)
}

export function useKeyboardNav() {
  return {
    keyboardMode: readonly(keyboardMode),
    navSeq: () => navSeq,
    closeLayers,
    focusedKey,
    focusKey,
    focusSection,
    focusSectionSoon,
    focusEntry,
    focusNext,
    neighborKey,
  }
}
