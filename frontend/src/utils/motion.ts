export type Spring = {duration: number; easing: string}

export const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches

// The --spring-* curves of theme.css, for element.animate().
export function spring(name: 'press' | 'pop' | 'slide' | 'land' | 'sheet' | 'page' | 'fast'): Spring {
  const css = getComputedStyle(document.documentElement)
  return {duration: parseFloat(css.getPropertyValue(`--spring-${name}-ms`)), easing: css.getPropertyValue(`--spring-${name}`).trim()}
}

export const EMPHASIZED_DECELERATE = 'cubic-bezier(0.05, 0.7, 0.1, 1)'
export const EMPHASIZED = 'cubic-bezier(0.2, 0, 0, 1)'

const RIPPLE_MS = 1000

export function pop(el: Element | null | undefined, from: number | string, delay = 0) {
  if (el && !reducedMotion()) el.animate([{scale: from}, {scale: 1}], {...spring('pop'), delay})
}

export const squash = (el: Element | null | undefined, delay = 0) => pop(el, '1.2 0.8', delay)

export function ripple(el: Element, options: {delay?: number; pseudoElement?: string} = {}): Animation | undefined {
  if (reducedMotion()) return
  return el.animate([{scale: 1, opacity: 0.6}, {scale: 3.2, opacity: 0}], {duration: RIPPLE_MS, easing: EMPHASIZED, fill: 'both', ...options})
}
