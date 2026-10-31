export type Spring = {duration: number; easing: string}

// Share of its full length each kind of animation runs for.
export const LONG_PACE = 0.5
export const DELAY_PACE = 0.4
export const SPRING_PACE = 0.65

const SPRING_MS = {press: 755, pop: 635, slide: 515, land: 550, sheet: 340, page: 375, fast: 350}

export const paced = (ms: number) => ms * LONG_PACE
export const delayed = (ms: number) => ms * DELAY_PACE

// CSS reads every pace from here; the build rewrites values kept in stylesheets, so they can't be read back.
export function applyPaces(root: HTMLElement) {
  root.style.setProperty('--pace-long', String(LONG_PACE))
  root.style.setProperty('--pace-delay', String(DELAY_PACE))
  for (const [name, ms] of Object.entries(SPRING_MS)) root.style.setProperty(`--spring-${name}-ms`, `${ms * SPRING_PACE}ms`)
}

export const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches

// The --spring-* curves of theme.css, for element.animate().
export function spring(name: keyof typeof SPRING_MS): Spring {
  const css = getComputedStyle(document.documentElement)
  return {duration: SPRING_MS[name] * SPRING_PACE, easing: css.getPropertyValue(`--spring-${name}`).trim()}
}

export const EMPHASIZED_DECELERATE = 'cubic-bezier(0.05, 0.7, 0.1, 1)'
export const EMPHASIZED = 'cubic-bezier(0.2, 0, 0, 1)'

const RIPPLE_MS = paced(870)

export function pop(el: Element | null | undefined, from: number | string, delay = 0) {
  if (el && !reducedMotion()) el.animate([{scale: from}, {scale: 1}], {...spring('pop'), delay})
}

export const squash = (el: Element | null | undefined, delay = 0) => pop(el, '1.2 0.8', delay)

export function ripple(el: Element, options: {delay?: number; pseudoElement?: string} = {}): Animation | undefined {
  if (reducedMotion()) return
  return el.animate([{scale: 1, opacity: 0.6}, {scale: 3.2, opacity: 0}], {duration: RIPPLE_MS, easing: EMPHASIZED, fill: 'both', ...options})
}
