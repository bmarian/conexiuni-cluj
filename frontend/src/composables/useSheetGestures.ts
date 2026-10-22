import {onMounted, onUnmounted, type Ref} from 'vue'
import {isTypingTarget} from '@/utils/keyboardFocus.ts'

export type SheetSnap = 'minimized' | 'half' | 'fullscreen'

const SNAPS: SheetSnap[] = ['fullscreen', 'half', 'minimized']
// Same as Sheet.kt.
const FLING_PX_PER_MS = 0.4
const REVERSE_SLOP_PX = 32
const TOUCH_SLOP_PX = 8
const VELOCITY_WINDOW_MS = 100

interface Sample {
  t: number
  y: number
}

interface Gesture {
  x0: number
  y0: number
  lastY: number
  mode: 'pending' | 'sheet' | 'off'
  scroller: HTMLElement | null
  top: number
  samples: Sample[]
}

export function useSheetGestures(options: {
  sheet: Ref<HTMLElement | null>
  enabled: Ref<boolean>
  state: Ref<SheetSnap>
  dragging: Ref<boolean>
  topFor: (snap: SheetSnap) => number
  onMove: (top: number) => void
  held?: () => boolean
}) {
  const {sheet, enabled, state, dragging, topFor, onMove, held} = options
  let gesture: Gesture | null = null
  let settling = false
  let settleTimer: ReturnType<typeof setTimeout> | null = null
  let wheelDirection = 0

  const fullTop = () => topFor('fullscreen')
  const minTop = () => topFor('minimized')
  const clampTop = (top: number) => Math.min(minTop(), Math.max(fullTop(), top))

  function renderedTop(): number {
    const el = sheet.value
    if (!el) return topFor(state.value)
    const transform = getComputedStyle(el).transform
    return transform && transform !== 'none' ? new DOMMatrix(transform).m42 : topFor(state.value)
  }

  function place(top: number) {
    const el = sheet.value
    if (!el) return
    el.style.transform = `translateY(${top}px)`
    onMove(top)
  }

  function begin(top: number) {
    const el = sheet.value
    if (!el) return
    settling = false
    dragging.value = true
    el.style.transition = 'none'
    place(top)
  }

  function velocityOf(samples: Sample[]): number {
    const last = samples.at(-1)
    const first = samples.find((s) => last && last.t - s.t <= VELOCITY_WINDOW_MS)
    if (!first || !last || last.t === first.t || performance.now() - last.t > VELOCITY_WINDOW_MS) return 0
    return (last.y - first.y) / (last.t - first.t)
  }

  function track(samples: Sample[], y: number) {
    const t = performance.now()
    samples.push({t, y})
    while (samples.length > 2 && t - samples[0]!.t > VELOCITY_WINDOW_MS) samples.shift()
  }

  // Port of SheetState.targetFor.
  function targetFor(top: number, velocity: number): SheetSnap {
    const at = SNAPS.map((snap) => ({snap, top: topFor(snap)}))
    const closest = at.reduce((a, b) => (Math.abs(b.top - top) < Math.abs(a.top - top) ? b : a)).snap
    if (Math.abs(velocity) < FLING_PX_PER_MS) return closest
    const down = velocity > 0
    const rest = topFor(state.value)
    const against = down ? top < rest - REVERSE_SLOP_PX : top > rest + REVERSE_SLOP_PX
    const from = against ? top : down ? Math.max(top, rest) : Math.min(top, rest)
    const ahead = at.filter((s) => (down ? s.top > from + 1 : s.top < from - 1))
    if (!ahead.length) return closest
    return ahead.reduce((a, b) => (Math.abs(b.top - from) < Math.abs(a.top - from) ? b : a)).snap
  }

  function settleTo(snap: SheetSnap) {
    const el = sheet.value
    dragging.value = false
    state.value = snap
    if (!el) return
    el.style.transition = ''
    const top = topFor(snap)
    const moving = Math.abs(renderedTop() - top) > 0.5
    place(top)
    if (!moving) return
    settling = true
    if (settleTimer) clearTimeout(settleTimer)
    settleTimer = setTimeout(() => {
      settling = false
    }, transitionMs(el))
  }

  function transitionMs(el: HTMLElement): number {
    const style = getComputedStyle(el)
    const names = style.transitionProperty.split(',').map((s) => s.trim())
    const durations = style.transitionDuration.split(',').map((s) => parseFloat(s) * (s.trim().endsWith('ms') ? 1 : 1000))
    const i = names.findIndex((n) => n === 'transform' || n === 'all')
    return durations[i >= 0 ? i % durations.length : 0] ?? 0
  }

  function releaseField() {
    const active = document.activeElement
    if (isTypingTarget(active) && sheet.value?.contains(active)) (active as HTMLElement).blur()
  }

  function scrollerOf(target: EventTarget | null): HTMLElement | null {
    const el = sheet.value
    if (!el) return null
    for (let node = target instanceof Element ? target : null; node && node !== el; node = node.parentElement) {
      if (!(node instanceof HTMLElement)) continue
      const overflow = getComputedStyle(node).overflowY
      if ((overflow === 'auto' || overflow === 'scroll') && node.scrollHeight > node.clientHeight) return node
    }
    return el.querySelector<HTMLElement>('.drawer-view')
  }

  function drag(g: Gesture, delta: number) {
    const scroller = g.scroller
    if (delta < 0) {
      const used = Math.max(delta, fullTop() - g.top)
      g.top += used
      if (scroller && delta - used < 0) scroller.scrollTop -= delta - used
    } else if (delta > 0) {
      const scrolled = scroller ? Math.min(delta, scroller.scrollTop) : 0
      if (scroller) scroller.scrollTop -= scrolled
      const rest = delta - scrolled
      if (rest > 0) {
        g.top = clampTop(g.top + rest)
        releaseField()
      }
    }
    place(g.top)
  }

  function onTouchStart(e: TouchEvent) {
    gesture = null
    if (!enabled.value || e.touches.length !== 1 || held?.()) return
    const target = e.target as Element | null
    if (target?.closest('.drawer-handle')) return
    const touch = e.touches[0]!
    gesture = {x0: touch.clientX, y0: touch.clientY, lastY: touch.clientY, mode: 'pending', scroller: scrollerOf(target), top: 0, samples: []}
  }

  function onTouchMove(e: TouchEvent) {
    const g = gesture
    if (!g || g.mode === 'off') return
    const touch = e.touches[0]
    if (!touch) return
    if (g.mode === 'pending') {
      const dx = touch.clientX - g.x0
      const dy = touch.clientY - g.y0
      if (Math.max(Math.abs(dx), Math.abs(dy)) < TOUCH_SLOP_PX) return
      // A row being reordered keeps the finger.
      if (Math.abs(dx) > Math.abs(dy) || document.querySelector('.drag-chosen')) {
        g.mode = 'off'
        return
      }
      const top = renderedTop()
      const atListTop = (g.scroller?.scrollTop ?? 0) <= 0
      const takes = dy < 0 ? top > fullTop() + 0.5 : atListTop && top < minTop() - 0.5
      if (!takes) {
        g.mode = 'off'
        return
      }
      g.mode = 'sheet'
      g.top = top
      g.lastY = touch.clientY
      begin(top)
    }
    e.preventDefault()
    track(g.samples, touch.clientY)
    drag(g, touch.clientY - g.lastY)
    g.lastY = touch.clientY
  }

  function onTouchEnd(e: TouchEvent) {
    const g = gesture
    gesture = null
    if (!g || g.mode !== 'sheet') return
    // No click on whatever slid under the finger.
    if (e.cancelable) e.preventDefault()
    settleTo(targetFor(g.top, velocityOf(g.samples)))
  }

  function onWheel(e: WheelEvent) {
    if (!enabled.value || e.ctrlKey || held?.()) return
    const scroller = scrollerOf(e.target)
    const unit = e.deltaMode === WheelEvent.DOM_DELTA_LINE
      ? parseFloat(getComputedStyle(scroller ?? document.documentElement).fontSize)
      : e.deltaMode === WheelEvent.DOM_DELTA_PAGE ? window.innerHeight : 1
    const dy = e.deltaY * unit
    if (!dy || Math.abs(e.deltaX * unit) > Math.abs(dy)) return
    const direction = Math.sign(dy)
    if (settling) {
      if (direction === wheelDirection) e.preventDefault()
      return
    }
    const top = renderedTop()
    const takes = dy > 0 ? top > fullTop() + 0.5 : (scroller?.scrollTop ?? 0) <= 0 && top < minTop() - 0.5
    if (!takes) return
    e.preventDefault()
    wheelDirection = direction
    if (dy < 0) releaseField()
    settleTo(targetFor(top, -direction * FLING_PX_PER_MS))
  }

  let handlePointer = -1
  let handleMoved = false
  let handleStartY = 0
  let handleStartTop = 0
  let handleSamples: Sample[] = []

  function onHandleDown(e: PointerEvent) {
    if (!enabled.value || (e.button !== 0 && e.pointerType === 'mouse')) return
    releaseField()
    handlePointer = e.pointerId
    handleMoved = false
    handleStartY = e.clientY
    handleStartTop = renderedTop()
    handleSamples = []
    begin(handleStartTop)
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  function onHandleMove(e: PointerEvent) {
    if (e.pointerId !== handlePointer) return
    const dy = e.clientY - handleStartY
    if (Math.abs(dy) >= TOUCH_SLOP_PX) handleMoved = true
    track(handleSamples, e.clientY)
    place(clampTop(handleStartTop + dy))
  }

  function onHandleUp(e: PointerEvent) {
    if (e.pointerId !== handlePointer) return
    handlePointer = -1
    const el = e.currentTarget as HTMLElement
    if (el.hasPointerCapture?.(e.pointerId)) el.releasePointerCapture(e.pointerId)
    if (!handleMoved) return settleTo(state.value === 'half' ? 'minimized' : 'half')
    settleTo(targetFor(renderedTop(), velocityOf(handleSamples)))
  }

  onMounted(() => {
    const el = sheet.value
    if (!el) return
    el.addEventListener('touchstart', onTouchStart, {passive: true})
    el.addEventListener('touchmove', onTouchMove, {passive: false})
    el.addEventListener('touchend', onTouchEnd, {passive: false})
    el.addEventListener('touchcancel', onTouchEnd, {passive: false})
    el.addEventListener('wheel', onWheel, {passive: false})
  })

  onUnmounted(() => {
    const el = sheet.value
    el?.removeEventListener('touchstart', onTouchStart)
    el?.removeEventListener('touchmove', onTouchMove)
    el?.removeEventListener('touchend', onTouchEnd)
    el?.removeEventListener('touchcancel', onTouchEnd)
    el?.removeEventListener('wheel', onWheel)
    if (settleTimer) clearTimeout(settleTimer)
  })

  return {onHandleDown, onHandleMove, onHandleUp}
}
