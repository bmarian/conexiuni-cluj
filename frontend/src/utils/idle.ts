// Runs `step` in the browser's idle time until it returns false; the returned function stops it.
export function whileIdle(step: () => boolean): () => void {
  const idle = typeof requestIdleCallback === 'function'
  let stopped = false
  let handle = 0
  const run = (deadline?: IdleDeadline) => {
    do {
      if (stopped || !step()) return
    } while (deadline && deadline.timeRemaining() > 0)
    handle = idle ? requestIdleCallback(run) : window.setTimeout(run)
  }
  handle = idle ? requestIdleCallback(run) : window.setTimeout(run)
  return () => {
    stopped = true
    if (idle) cancelIdleCallback(handle)
    else clearTimeout(handle)
  }
}
