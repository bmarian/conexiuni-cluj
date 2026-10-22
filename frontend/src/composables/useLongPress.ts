export const HOLD_MS = 500
const MOVE_TOLERANCE_PX = 10

export function useLongPress<T>(onHold: (item: T, x: number, y: number) => void) {
  let timer: ReturnType<typeof setTimeout> | null = null
  let startX = 0
  let startY = 0
  let held = false

  function cancel() {
    if (timer) clearTimeout(timer)
    timer = null
  }

  function bind(item: T) {
    return {
      class: 'hold-target',
      onPointerdown(e: PointerEvent) {
        held = false
        if (e.pointerType === 'mouse') return
        startX = e.clientX
        startY = e.clientY
        cancel()
        timer = setTimeout(() => {
          held = true
          navigator.vibrate?.(15)
          onHold(item, startX, startY)
        }, HOLD_MS)
      },
      onPointermove(e: PointerEvent) {
        if (timer && Math.hypot(e.clientX - startX, e.clientY - startY) > MOVE_TOLERANCE_PX) cancel()
      },
      onPointerup: cancel,
      onPointercancel: cancel,
      onPointerleave: cancel,
      onContextmenu(e: MouseEvent) {
        e.preventDefault()
        // Touch browsers also send this on hold.
        const touch = timer !== null || held || (e as PointerEvent).pointerType === 'touch'
        cancel()
        if (held) return
        // A hold's lift may still fire a click.
        held = touch
        onHold(item, e.clientX, e.clientY)
      },
    }
  }

  function click(handler: () => void) {
    if (held) {
      held = false
      return
    }
    handler()
  }

  function swallow(e: Event) {
    if (!held) return
    held = false
    e.preventDefault()
    e.stopPropagation()
  }

  return {bind, click, swallow}
}
