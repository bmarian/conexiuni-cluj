export type Spring = {duration: number; easing: string}

// Share of its full length each kind of animation runs for. main.ts hands them to the CSS.
export const LONG_PACE = 0.5
export const DELAY_PACE = 0.4
export const SPRING_PACE = 0.65

export const paced = (ms: number) => ms * LONG_PACE
export const delayed = (ms: number) => ms * DELAY_PACE

export const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches

// The --spring-* curves of theme.css, for element.animate().
export function spring(name: 'press' | 'pop' | 'slide' | 'land' | 'sheet' | 'page' | 'fast'): Spring {
  const css = getComputedStyle(document.documentElement)
  const full = /[\d.]+(?=ms)/.exec(css.getPropertyValue(`--spring-${name}-ms`))?.[0]
  return {duration: Number(full) * SPRING_PACE, easing: css.getPropertyValue(`--spring-${name}`).trim()}
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
